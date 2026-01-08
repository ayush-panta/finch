// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package vm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/runfinch/common-tests/command"
	"github.com/runfinch/common-tests/ffs"
	"github.com/runfinch/common-tests/fnet"
	"github.com/runfinch/common-tests/option"
)

// RegistryInfo contains registry connection details
type RegistryInfo struct {
	URL      string
	Username string
	Password string
}

// setupCredentialEnvironment creates a fresh credential store environment for testing
func setupCredentialEnvironment() func() {
	if os.Getenv("CI") == "true" {
		// Create fresh keychain for macOS CI
		homeDir, err := os.UserHomeDir()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		keychainsDir := filepath.Join(homeDir, "Library", "Keychains")
		loginKeychainPath := filepath.Join(keychainsDir, "login.keychain-db")
		keychainPassword := "test-password"

		// Remove existing keychain if present
		exec.Command("security", "delete-keychain", loginKeychainPath).Run()

		// Create Keychains directory
		err = os.MkdirAll(keychainsDir, 0755)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		// Create and setup keychain
		exec.Command("security", "create-keychain", "-p", keychainPassword, loginKeychainPath).Run()
		exec.Command("security", "unlock-keychain", "-p", keychainPassword, loginKeychainPath).Run()
		exec.Command("security", "list-keychains", "-s", loginKeychainPath, "/Library/Keychains/System.keychain").Run()
		exec.Command("security", "default-keychain", "-s", loginKeychainPath).Run()

		// Return cleanup function
		return func() {
			exec.Command("security", "delete-keychain", loginKeychainPath).Run()
		}
	}
	return func() {}
}

// setupFreshFinchConfig creates/replaces ~/.finch/config.json with credential helper configured
func setupFreshFinchConfig() string {
	homeDir, err := os.UserHomeDir()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	finchDir := filepath.Join(homeDir, ".finch")
	err = os.MkdirAll(finchDir, 0755)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	configPath := filepath.Join(finchDir, "config.json")
	configContent := `{"credsStore": "osxkeychain"}`
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return configPath
}

// testNativeCredHelper tests native credential helper functionality.
var testNativeCredHelper = func(o *option.Option, installed bool) {
	ginkgo.Describe("Native Credential Helper", func() {

		ginkgo.It("should work with local registry using native credential helper", func() {
			// Clean config and setup fresh environment
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup fresh finch config with native credential helper
			configPath := setupFreshFinchConfig()

			// Setup local authenticated registry - inline like finch config test
			filename := "htpasswd"
			registryImage := "public.ecr.aws/docker/library/registry:2"
			registryContainer := "auth-registry"
			//nolint:gosec // This password is only used for testing purpose.
			htpasswd := "testUser:$2y$05$wE0sj3r9O9K9q7R0MXcfPuIerl/06L1IsxXkCuUr3QZ8lHWwicIdS"
			htpasswdDir := filepath.Dir(ffs.CreateTempFile(filename, htpasswd))
			ginkgo.DeferCleanup(os.RemoveAll, htpasswdDir)
			port := fnet.GetFreePort()
			containerID := command.StdoutStr(o, "run",
				"-dp", fmt.Sprintf("%d:5000", port),
				"--name", registryContainer,
				"-v", fmt.Sprintf("%s:/auth", htpasswdDir),
				"-e", "REGISTRY_AUTH=htpasswd",
				"-e", "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm",
				"-e", fmt.Sprintf("REGISTRY_AUTH_HTPASSWD_PATH=/auth/%s", filename),
				registryImage)
			ginkgo.DeferCleanup(command.Run, o, "rmi", "-f", registryImage)
			ginkgo.DeferCleanup(command.Run, o, "rm", "-f", registryContainer)
			for command.StdoutStr(o, "inspect", "-f", "{{.State.Running}}", containerID) != "true" {
				time.Sleep(1 * time.Second)
			}
			time.Sleep(10 * time.Second)
			registry := fmt.Sprintf(`localhost:%d`, port)

			// Pull a base image first
			baseImage := "public.ecr.aws/docker/library/alpine:latest"
			command.Run(o, "pull", baseImage)

			// Show images before tagging
			fmt.Printf("Images BEFORE tagging:\n")
			imagesResult := command.New(o, "images").WithoutCheckingExitCode().Run()
			fmt.Printf("%s\n", string(imagesResult.Out.Contents()))

			// Tag and push to local registry to test credentials
			testImageTag := fmt.Sprintf("%s/test-native-creds:latest", registry)
			command.Run(o, "tag", baseImage, testImageTag)

			// Show images after tagging
			fmt.Printf("Images AFTER tagging:\n")
			imagesResult = command.New(o, "images").WithoutCheckingExitCode().Run()
			fmt.Printf("%s\n", string(imagesResult.Out.Contents()))

			// Print config BEFORE login
			configContent, err := os.ReadFile(filepath.Clean(configPath))
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			fmt.Printf("config.json BEFORE login:\n%s\n", string(configContent))

			// Login to registry - this should store credentials in native keychain
			command.New(o, "login", registry, "-u", "testUser", "-p", "testPassword").Run()

			// Print config AFTER login
			configContent, err = os.ReadFile(filepath.Clean(configPath))
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			fmt.Printf("config.json AFTER login:\n%s\n", string(configContent))

			// Test native keychain directly AFTER login
			fmt.Printf("Testing native keychain AFTER login:\n")
			keychainCmd := exec.Command("docker-credential-osxkeychain", "get")
			keychainCmd.Stdin = strings.NewReader(registry)
			keychainOutput, keychainErr := keychainCmd.CombinedOutput()
			if keychainErr != nil {
				fmt.Printf("Keychain error: %v\n", keychainErr)
			} else {
				fmt.Printf("Keychain output: %s\n", string(keychainOutput))
			}

			// Push image - this should use native credential helper via socket
			fmt.Printf("Pushing image to registry...\n")
			command.Run(o, "push", testImageTag)

			// Show images after push
			fmt.Printf("Images AFTER push:\n")
			imagesResult = command.New(o, "images").WithoutCheckingExitCode().Run()
			fmt.Printf("%s\n", string(imagesResult.Out.Contents()))

			// Remove local image
			fmt.Printf("Removing local image...\n")
			command.Run(o, "rmi", testImageTag)

			// Show images after removal
			fmt.Printf("Images AFTER removal:\n")
			imagesResult = command.New(o, "images").WithoutCheckingExitCode().Run()
			fmt.Printf("%s\n", string(imagesResult.Out.Contents()))

			// Pull image back - this should also use native credential helper
			fmt.Printf("Pulling image back from registry...\n")
			command.Run(o, "pull", testImageTag)

			// Show images after pull
			fmt.Printf("Images AFTER pull:\n")
			imagesResult = command.New(o, "images").WithoutCheckingExitCode().Run()
			fmt.Printf("%s\n", string(imagesResult.Out.Contents()))

			// Test run command specifically (the one that was failing)
			fmt.Printf("Testing run command (the main fix)...\n")
			runResult := command.New(o, "run", "--rm", testImageTag, "echo", "native-creds-test-success").WithoutCheckingExitCode().Run()
			fmt.Printf("Run result: exit=%d, output=%s\n", runResult.ExitCode(), string(runResult.Out.Contents()))
			gomega.Expect(runResult.ExitCode()).To(gomega.Equal(0), "Run command should succeed")

			// Print config BEFORE logout
			configContent, err = os.ReadFile(filepath.Clean(configPath))
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			fmt.Printf("config.json BEFORE logout:\n%s\n", string(configContent))

			// Cleanup
			fmt.Printf("Logging out from registry...\n")
			command.Run(o, "logout", registry)

			// Print config AFTER logout
			configContent, err = os.ReadFile(filepath.Clean(configPath))
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			fmt.Printf("config.json AFTER logout:\n%s\n", string(configContent))

			// Test native keychain directly AFTER logout
			fmt.Printf("Testing native keychain AFTER logout:\n")
			keychainCmd = exec.Command("docker-credential-osxkeychain", "get")
			keychainCmd.Stdin = strings.NewReader(registry)
			keychainOutput, keychainErr = keychainCmd.CombinedOutput()
			if keychainErr != nil {
				fmt.Printf("Keychain error (expected): %v\n", keychainErr)
			} else {
				fmt.Printf("Keychain output: %s\n", string(keychainOutput))
			}

			// Test that registry blocks unauthenticated access
			fmt.Printf("Testing registry blocks unauthenticated access...\n")
			command.Run(o, "rmi", testImageTag) // Remove image first
			
			// Test 1: Try to pull the pushed image without credentials - should fail
			unauthPullResult := command.New(o, "pull", testImageTag).WithoutCheckingExitCode().Run()
			fmt.Printf("Unauthenticated pull result: exit=%d, stderr=%s\n", unauthPullResult.ExitCode(), string(unauthPullResult.Err.Contents()))
			gomega.Expect(unauthPullResult.ExitCode()).ToNot(gomega.Equal(0), "Registry should block unauthenticated pull")
			
			// Test 2: Try to push without credentials - should fail
			newImageTag := fmt.Sprintf("%s/test-push-unauth:latest", registry)
			command.Run(o, "tag", baseImage, newImageTag)
			unauthPushResult := command.New(o, "push", newImageTag).WithoutCheckingExitCode().Run()
			fmt.Printf("Unauthenticated push result: exit=%d, stderr=%s\n", unauthPushResult.ExitCode(), string(unauthPushResult.Err.Contents()))
			gomega.Expect(unauthPushResult.ExitCode()).ToNot(gomega.Equal(0), "Registry should block unauthenticated push")
			
			fmt.Printf("SUCCESS: Registry properly blocks unauthenticated access\n")
		})
	})
}
