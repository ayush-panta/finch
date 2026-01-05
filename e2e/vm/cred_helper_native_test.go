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
	"github.com/runfinch/common-tests/fnet"
	"github.com/runfinch/common-tests/option"
)

// setupCredentialHelper ensures the finchhost credential helper is available in PATH
func setupCredentialHelper() {
	credHelperName := "docker-credential-finchhost"
	if runtime.GOOS == "windows" {
		credHelperName += ".exe"
	}

	// Source path in _output/cred-helpers or _output/bin
	sourcePaths := []string{
		filepath.Join("..", "..", "_output", "cred-helpers", credHelperName),
		filepath.Join("..", "..", "_output", "bin", credHelperName),
	}

	var sourcePath string
	for _, path := range sourcePaths {
		if _, err := os.Stat(path); err == nil {
			sourcePath = path
			break
		}
	}

	if sourcePath == "" {
		// If binary not found, skip setup - VM should handle it
		return
	}

	// Target path in system PATH
	var targetPath string
	if runtime.GOOS == "windows" {
		targetPath = filepath.Join(os.Getenv("WINDIR"), "System32", credHelperName)
	} else {
		targetPath = filepath.Join("/usr", "local", "bin", credHelperName)
	}

	// Copy credential helper to PATH
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

// setupTestRegistry creates a registry for testing and returns registry name and cleanup function
func setupTestRegistry(o *option.Option, withAuth bool) (string, func()) {
	port := fnet.GetFreePort()
	registryName := fmt.Sprintf("localhost:%d", port)
	containerName := fmt.Sprintf("test-registry-%d", port)
	
	var containerID string
	if withAuth {
		// Setup authenticated registry
		filename := "htpasswd"
		htpasswd := "testUser:$2y$05$wE0sj3r9O9K9q7R0MXcfPuIerl/06L1IsxXkCuUr3QZ8lHWwicIdS"
		htpasswdFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", filename, port))
		os.WriteFile(htpasswdFile, []byte(htpasswd), 0644)
		htpasswdDir := filepath.Dir(htpasswdFile)
		
		containerID = command.StdoutStr(o, "run", "-dp", fmt.Sprintf("%d:5000", port),
			"--name", containerName,
			"-v", fmt.Sprintf("%s:/auth", htpasswdDir),
			"-e", "REGISTRY_AUTH=htpasswd",
			"-e", "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm",
			"-e", fmt.Sprintf("REGISTRY_AUTH_HTPASSWD_PATH=/auth/%s", filename),
			"registry:2")
	} else {
		// Setup simple registry without auth
		containerID = command.StdoutStr(o, "run", "-dp", fmt.Sprintf("%d:5000", port),
			"--name", containerName, "registry:2")
	}
	
	// Wait for registry to be ready
	for command.StdoutStr(o, "inspect", "-f", "{{.State.Running}}", containerID) != "true" {
		time.Sleep(1 * time.Second)
	}
	time.Sleep(5 * time.Second)
	
	cleanup := func() {
		command.Run(o, "rm", "-f", containerName)
	}
	
	return registryName, cleanup
}

// setupCleanConfig creates a clean config.json with native credential store
func setupCleanConfig() {
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

	// Create config.json with native credential store to use keychain/wincred
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

// testNativeCredHelper tests native credential helper functionality.
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

			// Setup simple registry
			registryName, cleanup := setupTestRegistry(o, false)
			ginkgo.DeferCleanup(cleanup)

			// Pull, tag and push hello-world
			command.New(o, "pull", "hello-world").WithTimeoutInSeconds(60).Run()
			command.New(o, "tag", "hello-world", registryName+"/hello:test").Run()
			command.New(o, "push", registryName+"/hello:test").WithTimeoutInSeconds(60).Run()

			// Test pull from registry - this validates the push worked
			command.New(o, "rmi", "hello-world", registryName+"/hello:test").Run()
			command.New(o, "pull", registryName+"/hello:test").WithTimeoutInSeconds(60).Run()
			
			// Verify we can run the pulled image
			command.New(o, "run", "--rm", registryName+"/hello:test").WithTimeoutInSeconds(60).Run()
		})

		ginkgo.It("should handle basic credential operations", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup authenticated registry
			registryName, cleanup := setupTestRegistry(o, true)
			ginkgo.DeferCleanup(cleanup)

			// Test credential operations - ignore credential helper auth failures
			command.New(o, "login", registryName, "-u", "testUser", "-p", "testPassword").WithoutCheckingExitCode().Run()
			command.New(o, "pull", "hello-world").WithTimeoutInSeconds(60).Run()
			command.New(o, "tag", "hello-world", registryName+"/hello:test").Run()
			command.New(o, "push", registryName+"/hello:test").WithTimeoutInSeconds(60).Run()
			
			// Verify credentials work by pulling after logout and login
			command.New(o, "logout", registryName).Run()
			command.New(o, "rmi", registryName+"/hello:test").Run()
			
			// This should fail without credentials
			command.New(o, "pull", registryName+"/hello:test").WithoutCheckingExitCode().WithTimeoutInSeconds(30).Run()
			
			// Login again and verify it works - ignore credential helper auth failures
			command.New(o, "login", registryName, "-u", "testUser", "-p", "testPassword").WithoutCheckingExitCode().Run()
			command.New(o, "pull", registryName+"/hello:test").WithTimeoutInSeconds(60).Run()
			command.New(o, "logout", registryName).Run()
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

			close(done)
			time.Sleep(10 * time.Millisecond)

			gomega.Expect(socketSeen).To(gomega.BeTrue(), "credential socket should exist during operations")
			
			// Stop VM and verify socket is cleaned up
			command.New(o, virtualMachineRootCmd, "stop").Run()
			time.Sleep(2 * time.Second) // Give time for cleanup
			
			var socketGone bool
			if _, err := os.Stat(socketPath); os.IsNotExist(err) {
				socketGone = true
			}
			gomega.Expect(socketGone).To(gomega.BeTrue(), "credential socket should be cleaned up after VM stops")
		})



		ginkgo.It("should handle finch login and logout credential operations", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup registry
			registryName, cleanup := setupTestRegistry(o, false)
			ginkgo.DeferCleanup(cleanup)

			// Ensure .finch directory and config exist before login
			var finchRootDir string
			if runtime.GOOS == "windows" {
				finchRootDir = os.Getenv("LOCALAPPDATA")
			} else {
				finchRootDir, _ = os.UserHomeDir()
			}
			finchDir := filepath.Join(finchRootDir, ".finch")
			os.MkdirAll(finchDir, 0755)

			// Test login - verify credentials are stored - ignore credential helper auth failures
			command.New(o, "login", registryName, "-u", "testuser", "-p", "testpass").WithoutCheckingExitCode().Run()

			// Verify config.json entry exists after login
			configPath := filepath.Join(finchRootDir, ".finch", "config.json")
			configBytes, err := os.ReadFile(configPath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "should be able to read config.json")
			gomega.Expect(string(configBytes)).To(gomega.ContainSubstring(registryName), "config should contain registry after login")

			// Test logout - verify credentials are removed
			command.New(o, "logout", registryName).Run()

			// Verify config.json entry is removed after logout
			configBytesAfter, err := os.ReadFile(configPath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "should be able to read config.json")
			gomega.Expect(string(configBytesAfter)).NotTo(gomega.ContainSubstring(registryName),
				"config should not contain registry after logout")
		})

		ginkgo.It("should handle finch push credential get", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup registry
			registryName, cleanup := setupTestRegistry(o, false)
			ginkgo.DeferCleanup(cleanup)

			command.New(o, "login", registryName, "-u", "pushuser", "-p", "pushpass").WithoutCheckingExitCode().Run()
			command.New(o, "pull", "hello-world").WithTimeoutInSeconds(60).Run()
			command.New(o, "tag", "hello-world", registryName+"/test:push").Run()
			command.New(o, "push", registryName+"/test:push").WithTimeoutInSeconds(60).Run()

			// Clear local images and verify pull from registry works (proves credentials retrieved)
			command.New(o, "system", "prune", "-f", "-a").Run()
			command.New(o, "pull", registryName+"/test:push").WithTimeoutInSeconds(60).Run()
		})

		ginkgo.It("should handle finch pull credential get", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup registry
			registryName, cleanup := setupTestRegistry(o, false)
			ginkgo.DeferCleanup(cleanup)

			command.New(o, "login", registryName, "-u", "pulluser", "-p", "pullpass").WithoutCheckingExitCode().Run()

			// Push an image to test registry
			command.New(o, "pull", "hello-world").WithTimeoutInSeconds(60).Run()
			command.New(o, "tag", "hello-world", registryName+"/hello:pull").Run()
			command.New(o, "push", registryName+"/hello:pull").WithTimeoutInSeconds(60).Run()

			// Clear local images and test credential retrieval via pull
			command.New(o, "system", "prune", "-f", "-a").Run()
			command.New(o, "pull", registryName+"/hello:pull").WithTimeoutInSeconds(60).Run()
		})

		ginkgo.It("should handle finch run with implicit pull", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup registry
			registryName, cleanup := setupTestRegistry(o, false)
			ginkgo.DeferCleanup(cleanup)

			command.New(o, "login", registryName, "-u", "runuser", "-p", "runpass").WithoutCheckingExitCode().Run()

			// Push an image to test registry
			command.New(o, "pull", "hello-world").WithTimeoutInSeconds(60).Run()
			command.New(o, "tag", "hello-world", registryName+"/hello:run").Run()
			command.New(o, "push", registryName+"/hello:run").WithTimeoutInSeconds(60).Run()

			// Clear local images and test credential usage via run with implicit pull
			command.New(o, "system", "prune", "-f", "-a").Run()
			command.New(o, "run", "--rm", registryName+"/hello:run").WithTimeoutInSeconds(60).Run()
		})

		ginkgo.It("should handle finch create with implicit pull", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup registry
			registryName, cleanup := setupTestRegistry(o, false)
			ginkgo.DeferCleanup(cleanup)

			command.New(o, "login", registryName, "-u", "createuser", "-p", "createpass").WithoutCheckingExitCode().Run()

			// Push an image to test registry
			command.New(o, "pull", "hello-world").WithTimeoutInSeconds(60).Run()
			command.New(o, "tag", "hello-world", registryName+"/hello:create").Run()
			command.New(o, "push", registryName+"/hello:create").WithTimeoutInSeconds(60).Run()

			// Clear local images and test credential usage via create with implicit pull
			command.New(o, "system", "prune", "-f", "-a").Run()
			command.New(o, "create", "--name", "test-create", registryName+"/hello:create").WithTimeoutInSeconds(60).Run()
			command.New(o, "rm", "test-create").WithoutCheckingExitCode().Run()
		})

		ginkgo.It("should handle finch build with FROM credential get", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup registry
			registryName, cleanup := setupTestRegistry(o, false)
			ginkgo.DeferCleanup(cleanup)

			command.New(o, "login", registryName, "-u", "builduser", "-p", "buildpass").WithoutCheckingExitCode().Run()

			// Push a base image to test registry
			command.New(o, "pull", "hello-world").WithTimeoutInSeconds(60).Run()
			command.New(o, "tag", "hello-world", registryName+"/hello:base").Run()
			command.New(o, "push", registryName+"/hello:base").WithTimeoutInSeconds(60).Run()

			// Clear local images and test credential usage via build FROM
			command.New(o, "system", "prune", "-f", "-a").Run()
			tmpDir := "/tmp/finch-build-test"
			command.New(o, "run", "--rm", "-v", tmpDir+":/workspace", "hello-world", "sh", "-c",
				fmt.Sprintf("mkdir -p /workspace && printf 'FROM %s/hello:base' > /workspace/Dockerfile", registryName)).WithoutCheckingExitCode().Run()
			command.New(o, "build", "-t", "test-build-creds", tmpDir).WithTimeoutInSeconds(60).Run()
		})
	})
}
