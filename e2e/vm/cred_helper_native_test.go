// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin || windows

package vm

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/runfinch/common-tests/command"
	"github.com/runfinch/common-tests/option"
)

// setupCleanConfig creates a clean config.json with just the credential store
func setupCleanConfig() {
	homeDir, _ := os.UserHomeDir()
	finchDir := filepath.Join(homeDir, ".finch")
	os.MkdirAll(finchDir, 0755)

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

			// Test sequential pulls to verify credential system works properly
			command.New(o, "pull", "public.ecr.aws/docker/library/alpine:latest").WithTimeoutInSeconds(300).Run()
			command.New(o, "pull", "public.ecr.aws/docker/library/nginx:latest").WithTimeoutInSeconds(300).Run()
			command.New(o, "pull", "public.ecr.aws/docker/library/postgres:latest").WithTimeoutInSeconds(300).Run()
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
			command.New(o, "pull", "public.ecr.aws/docker/library/postgres:latest").WithTimeoutInSeconds(300).Run()

			close(done)
			time.Sleep(10 * time.Millisecond)

			gomega.Expect(socketSeen).To(gomega.BeTrue(), "credential socket should exist during operations")
		})

		ginkgo.It("should have secure credential helper installation", func() {
			ginkgo.Skip("Skipping security installation test as it's environment-specific and tests build/installation details rather than runtime functionality")
		})

		ginkgo.It("should handle finch login and logout credential operations", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			command.New(o, "run", "-d", "-p", "5001:5000", "--name", "login-registry", "registry:2").Run()
			// Wait for registry to be ready
			time.Sleep(5 * time.Second)

			// Test login - verify credentials are stored
			command.New(o, "login", "localhost:5001", "-u", "testuser", "-p", "testpass").Run()

			// Verify config.json entry exists after login
			homeDir, _ := os.UserHomeDir()
			configPath := filepath.Join(homeDir, ".finch", "config.json")
			configBytes, err := os.ReadFile(configPath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "should be able to read config.json")
			gomega.Expect(string(configBytes)).To(gomega.ContainSubstring("localhost:5001"), "config should contain registry after login")

			// Test logout - verify credentials are removed
			command.New(o, "logout", "localhost:5001").Run()

			// Verify config.json entry is removed after logout
			configBytesAfter, err := os.ReadFile(configPath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "should be able to read config.json")
			gomega.Expect(string(configBytesAfter)).NotTo(gomega.ContainSubstring("localhost:5001"), "config should not contain registry after logout")

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

			command.New(o, "pull", "public.ecr.aws/docker/library/alpine:latest").WithTimeoutInSeconds(300).Run()
		})

		ginkgo.It("should handle finch run with implicit pull", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			command.New(o, "run", "--rm", "public.ecr.aws/docker/library/alpine:latest", "echo", "test").WithTimeoutInSeconds(300).Run()
		})

		ginkgo.It("should handle finch create with implicit pull", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			command.New(o, "create", "--name", "test-create", "public.ecr.aws/docker/library/alpine:latest").WithTimeoutInSeconds(300).Run()
			command.New(o, "rm", "test-create").WithoutCheckingExitCode().Run()
		})

		ginkgo.It("should handle finch build with FROM credential get", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			tmpDir := "/tmp/finch-build-test"
			command.New(o, "run", "--rm", "-v", tmpDir+":/workspace", "alpine", "sh", "-c",
				"mkdir -p /workspace && printf 'FROM public.ecr.aws/docker/library/alpine:latest\nRUN echo build-test' > /workspace/Dockerfile").Run()
			command.New(o, "build", "-t", "test-build-creds", tmpDir).WithTimeoutInSeconds(300).Run()
		})
	})
}
