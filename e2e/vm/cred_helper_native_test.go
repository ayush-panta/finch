// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin || windows

package vm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/runfinch/common-tests/command"
	"github.com/runfinch/common-tests/option"
)

// setupCredentialHelper ensures the native credential helper is available in PATH
func setupCredentialHelper() {
	var credHelperName string
	if runtime.GOOS == "windows" {
		credHelperName = "docker-credential-wincred.exe"
	} else {
		credHelperName = "docker-credential-osxkeychain"
	}

	// Source path in _output/cred-helpers
	sourcePath := filepath.Join("..", "..", "_output", "cred-helpers", credHelperName)

	// Target path in system PATH
	var targetPath string
	if runtime.GOOS == "windows" {
		targetPath = filepath.Join(os.Getenv("WINDIR"), "System32", credHelperName)
	} else {
		targetPath = filepath.Join("/usr", "local", "bin", credHelperName)
	}

	// Copy credential helper to PATH if source exists
	if _, err := os.Stat(sourcePath); err == nil {
		// Use command execution for proper permissions
		if runtime.GOOS == "windows" {
			// Windows copy
			sourceData, err := os.ReadFile(sourcePath)
			if err == nil {
				os.WriteFile(targetPath, sourceData, 0755)
			}
		} else {
			// macOS/Linux copy with sudo
			exec.Command("sudo", "cp", sourcePath, targetPath).Run()
			exec.Command("sudo", "chmod", "+x", targetPath).Run()
		}
	}
}

// setupCleanConfig creates a clean config.json with just the credential store
func setupCleanConfig() {
	// Setup credential helper in PATH
	setupCredentialHelper()

	var finchRootDir string
	if runtime.GOOS == "windows" {
		finchRootDir = os.Getenv("LOCALAPPDATA")
	} else {
		finchRootDir, _ = os.UserHomeDir()
	}
	finchDir := filepath.Join(finchRootDir, ".finch")
	os.MkdirAll(finchDir, 0755)

	// Set DOCKER_CONFIG to point to .finch directory
	os.Setenv("DOCKER_CONFIG", finchDir)

	var credStore string
	if runtime.GOOS == "windows" {
		credStore = "wincred"
	} else {
		credStore = "osxkeychain"
	}
	configContent := fmt.Sprintf(`{"credsStore":"%s"}`, credStore)
	configPath := filepath.Join(finchDir, "config.json")
	os.WriteFile(configPath, []byte(configContent), 0644)
}

// testNativeCredHelper is unused in current test suite but kept for future use.
//
//nolint:unused
var testNativeCredHelper = func(o *option.Option, installed bool) {
	ginkgo.Describe("Native Credential Helper", func() {
		ginkgo.BeforeEach(func() {
			// Clean config before each test
			setupCleanConfig()
		})
		ginkgo.It("should support registry workflow with build and push", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup local registry
			registryPort := "5000"
			registryName := "localhost:" + registryPort
			command.New(o, "run", "-d", "-p", registryPort+":5000", "--name", "test-registry", "registry:2").Run()

			// Login to local registry
			command.New(o, "login", registryName, "-u", "test", "-p", "test").WithoutCheckingExitCode().Run()

			// Pull, tag and push single image
			command.New(o, "pull", "public.ecr.aws/docker/library/alpine:latest").WithTimeoutInSeconds(300).Run()
			command.New(o, "tag", "public.ecr.aws/docker/library/alpine:latest", registryName+"/alpine:test").Run()
			command.New(o, "push", registryName+"/alpine:test").WithTimeoutInSeconds(300).Run()

			// Build and push dockerfile
			tmpDir := "/tmp/finch-test-build"
			command.New(o, "run", "--rm", "-v", tmpDir+":/workspace", "alpine", "sh", "-c",
				"mkdir -p /workspace && echo 'FROM alpine\nRUN echo test' > /workspace/Dockerfile").Run()
			command.New(o, "build", "-t", registryName+"/test-build", tmpDir).Run()
			command.New(o, "push", registryName+"/test-build").WithTimeoutInSeconds(300).Run()

			// Clear local images and test pull from registry
			command.New(o, "system", "prune", "-f", "-a").Run()
			command.New(o, "run", "--rm", registryName+"/alpine:test", "echo", "success").WithTimeoutInSeconds(300).Run()
			command.New(o, "run", "--rm", registryName+"/test-build", "echo", "build-success").WithTimeoutInSeconds(300).Run()

			// Logout and cleanup
			command.New(o, "logout", registryName).WithoutCheckingExitCode().Run()
			command.New(o, "stop", "test-registry").WithoutCheckingExitCode().Run()
			command.New(o, "rm", "test-registry").WithoutCheckingExitCode().Run()
		})

		ginkgo.It("should handle sequential credential operations", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup local registry for credential testing
			command.New(o, "run", "-d", "-p", "5003:5000", "--name", "seq-registry", "registry:2").Run()
			time.Sleep(5 * time.Second)
			command.New(o, "login", "localhost:5003", "-u", "sequser", "-p", "seqpass").Run()

			// Test sequential pulls with authentication
			command.New(o, "pull", "public.ecr.aws/docker/library/alpine:latest").WithTimeoutInSeconds(300).Run()
			command.New(o, "tag", "public.ecr.aws/docker/library/alpine:latest", "localhost:5003/alpine:seq").Run()
			command.New(o, "push", "localhost:5003/alpine:seq").WithTimeoutInSeconds(300).Run()

			command.New(o, "pull", "public.ecr.aws/docker/library/nginx:latest").WithTimeoutInSeconds(300).Run()
			command.New(o, "tag", "public.ecr.aws/docker/library/nginx:latest", "localhost:5003/nginx:seq").Run()
			command.New(o, "push", "localhost:5003/nginx:seq").WithTimeoutInSeconds(300).Run()

			command.New(o, "pull", "public.ecr.aws/docker/library/postgres:latest").WithTimeoutInSeconds(300).Run()
			command.New(o, "tag", "public.ecr.aws/docker/library/postgres:latest", "localhost:5003/postgres:seq").Run()
			command.New(o, "push", "localhost:5003/postgres:seq").WithTimeoutInSeconds(300).Run()

			// Cleanup
			command.New(o, "logout", "localhost:5003").WithoutCheckingExitCode().Run()
			command.New(o, "stop", "seq-registry").WithoutCheckingExitCode().Run()
			command.New(o, "rm", "seq-registry").WithoutCheckingExitCode().Run()
		})

		ginkgo.It("should create and cleanup credential socket", func() {
			socketPath := filepath.Join("..", "..", "_output", "lima", "data", "finch", "sock", "creds.sock")

			var socketSeen bool
			done := make(chan struct{})

			// Background watcher
			go func() {
				defer ginkgo.GinkgoRecover()
				for {
					select {
					case <-done:
						return
					default:
						if _, err := os.Stat(socketPath); err == nil {
							socketSeen = true
						}
					}
				}
			}()

			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup registry and login to trigger credential socket usage
			command.New(o, "run", "-d", "-p", "5004:5000", "--name", "socket-registry", "registry:2").Run()
			time.Sleep(5 * time.Second)
			command.New(o, "login", "localhost:5004", "-u", "sockuser", "-p", "sockpass").Run()
			command.New(o, "pull", "public.ecr.aws/docker/library/postgres:latest").WithTimeoutInSeconds(300).Run()

			close(done)
			time.Sleep(10 * time.Millisecond)

			gomega.Expect(socketSeen).To(gomega.BeTrue(), "credential socket should exist during operations")

			// Cleanup
			command.New(o, "logout", "localhost:5004").WithoutCheckingExitCode().Run()
			command.New(o, "stop", "socket-registry").WithoutCheckingExitCode().Run()
			command.New(o, "rm", "socket-registry").WithoutCheckingExitCode().Run()
		})



		ginkgo.It("should handle finch login and logout credential operations", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Ensure .finch directory and config exist before login
			var finchRootDir string
			if runtime.GOOS == "windows" {
				finchRootDir = os.Getenv("LOCALAPPDATA")
			} else {
				finchRootDir, _ = os.UserHomeDir()
			}
			finchDir := filepath.Join(finchRootDir, ".finch")
			os.MkdirAll(finchDir, 0755)

			command.New(o, "run", "-d", "-p", "5001:5000", "--name", "login-registry", "registry:2").Run()
			// Wait for registry to be ready
			time.Sleep(5 * time.Second)

			// Test login - verify credentials are stored
			command.New(o, "login", "localhost:5001", "-u", "testuser", "-p", "testpass").Run()

			// Verify config.json entry exists after login
			configPath := filepath.Join(finchRootDir, ".finch", "config.json")
			configBytes, err := os.ReadFile(configPath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "should be able to read config.json")
			gomega.Expect(string(configBytes)).To(gomega.ContainSubstring("localhost:5001"), "config should contain registry after login")

			// Test logout - verify credentials are removed
			command.New(o, "logout", "localhost:5001").Run()

			// Verify config.json entry is removed after logout
			configBytesAfter, err := os.ReadFile(configPath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "should be able to read config.json")
			gomega.Expect(string(configBytesAfter)).NotTo(gomega.ContainSubstring("localhost:5001"),
				"config should not contain registry after logout")

			command.New(o, "stop", "login-registry").WithoutCheckingExitCode().Run()
			command.New(o, "rm", "login-registry").WithoutCheckingExitCode().Run()
		})

		ginkgo.It("should handle finch push credential get", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			command.New(o, "run", "-d", "-p", "5002:5000", "--name", "push-registry", "registry:2").Run()
			// Wait for registry to be ready
			time.Sleep(5 * time.Second)
			command.New(o, "login", "localhost:5002", "-u", "pushuser", "-p", "pushpass").Run()
			command.New(o, "pull", "public.ecr.aws/docker/library/alpine:latest").WithTimeoutInSeconds(300).Run()
			command.New(o, "tag", "public.ecr.aws/docker/library/alpine:latest", "localhost:5002/test:push").Run()
			command.New(o, "push", "localhost:5002/test:push").WithTimeoutInSeconds(300).Run()

			// Clear local images and verify pull from registry works (proves credentials retrieved)
			command.New(o, "system", "prune", "-f", "-a").Run()
			command.New(o, "pull", "localhost:5002/test:push").WithTimeoutInSeconds(300).Run()

			command.New(o, "logout", "localhost:5002").WithoutCheckingExitCode().Run()
			command.New(o, "stop", "push-registry").WithoutCheckingExitCode().Run()
			command.New(o, "rm", "push-registry").WithoutCheckingExitCode().Run()
		})

		ginkgo.It("should handle finch pull credential get", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup registry and login to test credential retrieval
			command.New(o, "run", "-d", "-p", "5005:5000", "--name", "pull-registry", "registry:2").Run()
			time.Sleep(5 * time.Second)
			command.New(o, "login", "localhost:5005", "-u", "pulluser", "-p", "pullpass").Run()

			// Push an image to test registry
			command.New(o, "pull", "public.ecr.aws/docker/library/alpine:latest").WithTimeoutInSeconds(300).Run()
			command.New(o, "tag", "public.ecr.aws/docker/library/alpine:latest", "localhost:5005/alpine:pull").Run()
			command.New(o, "push", "localhost:5005/alpine:pull").WithTimeoutInSeconds(300).Run()

			// Clear local images and test credential retrieval via pull
			command.New(o, "system", "prune", "-f", "-a").Run()
			command.New(o, "pull", "localhost:5005/alpine:pull").WithTimeoutInSeconds(300).Run()

			// Cleanup
			command.New(o, "logout", "localhost:5005").WithoutCheckingExitCode().Run()
			command.New(o, "stop", "pull-registry").WithoutCheckingExitCode().Run()
			command.New(o, "rm", "pull-registry").WithoutCheckingExitCode().Run()
		})

		ginkgo.It("should handle finch run with implicit pull", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup registry and login to test credential usage in run
			command.New(o, "run", "-d", "-p", "5006:5000", "--name", "run-registry", "registry:2").Run()
			time.Sleep(5 * time.Second)
			command.New(o, "login", "localhost:5006", "-u", "runuser", "-p", "runpass").Run()

			// Push an image to test registry
			command.New(o, "pull", "public.ecr.aws/docker/library/alpine:latest").WithTimeoutInSeconds(300).Run()
			command.New(o, "tag", "public.ecr.aws/docker/library/alpine:latest", "localhost:5006/alpine:run").Run()
			command.New(o, "push", "localhost:5006/alpine:run").WithTimeoutInSeconds(300).Run()

			// Clear local images and test credential usage via run with implicit pull
			command.New(o, "system", "prune", "-f", "-a").Run()
			command.New(o, "run", "--rm", "localhost:5006/alpine:run", "echo", "test").WithTimeoutInSeconds(300).Run()

			// Cleanup
			command.New(o, "logout", "localhost:5006").WithoutCheckingExitCode().Run()
			command.New(o, "stop", "run-registry").WithoutCheckingExitCode().Run()
			command.New(o, "rm", "run-registry").WithoutCheckingExitCode().Run()
		})

		ginkgo.It("should handle finch create with implicit pull", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup registry and login to test credential usage in create
			command.New(o, "run", "-d", "-p", "5007:5000", "--name", "create-registry", "registry:2").Run()
			time.Sleep(5 * time.Second)
			command.New(o, "login", "localhost:5007", "-u", "createuser", "-p", "createpass").Run()

			// Push an image to test registry
			command.New(o, "pull", "public.ecr.aws/docker/library/alpine:latest").WithTimeoutInSeconds(300).Run()
			command.New(o, "tag", "public.ecr.aws/docker/library/alpine:latest", "localhost:5007/alpine:create").Run()
			command.New(o, "push", "localhost:5007/alpine:create").WithTimeoutInSeconds(300).Run()

			// Clear local images and test credential usage via create with implicit pull
			command.New(o, "system", "prune", "-f", "-a").Run()
			command.New(o, "create", "--name", "test-create", "localhost:5007/alpine:create").WithTimeoutInSeconds(300).Run()
			command.New(o, "rm", "test-create").WithoutCheckingExitCode().Run()

			// Cleanup
			command.New(o, "logout", "localhost:5007").WithoutCheckingExitCode().Run()
			command.New(o, "stop", "create-registry").WithoutCheckingExitCode().Run()
			command.New(o, "rm", "create-registry").WithoutCheckingExitCode().Run()
		})

		ginkgo.It("should handle finch build with FROM credential get", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup registry and login to test credential usage in build
			command.New(o, "run", "-d", "-p", "5008:5000", "--name", "build-registry", "registry:2").Run()
			time.Sleep(5 * time.Second)
			command.New(o, "login", "localhost:5008", "-u", "builduser", "-p", "buildpass").Run()

			// Push a base image to test registry
			command.New(o, "pull", "public.ecr.aws/docker/library/alpine:latest").WithTimeoutInSeconds(300).Run()
			command.New(o, "tag", "public.ecr.aws/docker/library/alpine:latest", "localhost:5008/alpine:base").Run()
			command.New(o, "push", "localhost:5008/alpine:base").WithTimeoutInSeconds(300).Run()

			// Clear local images and test credential usage via build FROM
			command.New(o, "system", "prune", "-f", "-a").Run()
			tmpDir := "/tmp/finch-build-test"
			command.New(o, "run", "--rm", "-v", tmpDir+":/workspace", "alpine", "sh", "-c",
				"mkdir -p /workspace && printf 'FROM localhost:5008/alpine:base\nRUN echo build-test' > /workspace/Dockerfile").Run()
			command.New(o, "build", "-t", "test-build-creds", tmpDir).WithTimeoutInSeconds(300).Run()

			// Cleanup
			command.New(o, "logout", "localhost:5008").WithoutCheckingExitCode().Run()
			command.New(o, "stop", "build-registry").WithoutCheckingExitCode().Run()
			command.New(o, "rm", "build-registry").WithoutCheckingExitCode().Run()
		})
	})
}
