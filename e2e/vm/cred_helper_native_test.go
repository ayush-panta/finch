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

// setupCleanFinchConfig creates/replaces ~/.finch/config.json with credential helper configured
func setupCleanFinchConfig() string {
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

// setupCredentialEnvironment creates a fresh credential store environment for testing
func setupCredentialEnvironment() func() {
	// Check for GitHub Actions environment
	if os.Getenv("GITHUB_ACTIONS") == "true" {
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

// testNativeCredHelper tests native credential helper functionality.
var testNativeCredHelper = func(o *option.Option, installed bool) {
	ginkgo.Describe("Native Credential Helper", func() {

		ginkgo.It("comprehensive native credential helper workflow", func() {
			// Setup credential environment for CI
			cleanupCredentials := setupCredentialEnvironment()
			ginkgo.DeferCleanup(cleanupCredentials)

			// Setup Phase
			ginkgo.By("Setting up VM and registry")
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()
			configPath := setupCleanFinchConfig()

			// Setup authenticated registry
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
			fmt.Printf("Registry running at %s\n", registry)

			// 1. Pre-auth baseline: Pull and tag small image
			ginkgo.By("Testing credential-less operations")
			baseImage := "public.ecr.aws/docker/library/alpine:latest"
			command.Run(o, "pull", baseImage)
			testImageTag := fmt.Sprintf("%s/test-creds:latest", registry)
			command.Run(o, "tag", baseImage, testImageTag)

			// 2. Pre-login state: Verify registry not in auths
			ginkgo.By("Verifying pre-login state")
			configContent, err := os.ReadFile(filepath.Clean(configPath))
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			fmt.Printf("Config before login: %s\n", string(configContent))
			gomega.Expect(string(configContent)).ToNot(gomega.ContainSubstring(registry), "Registry should not be in auths before login")

			// 3. Login
			ginkgo.By("Logging in to registry")
			command.Run(o, "login", registry, "-u", "testUser", "-p", "testPassword")

			// 4. Post-login verification
			ginkgo.By("Verifying post-login credential storage")
			configContent, err = os.ReadFile(filepath.Clean(configPath))
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			fmt.Printf("Config after login: %s\n", string(configContent))
			gomega.Expect(string(configContent)).To(gomega.ContainSubstring(registry), "Registry should appear in auths after login")
			gomega.Expect(string(configContent)).ToNot(gomega.ContainSubstring("testPassword"), "Password should not be stored in config")

			// Verify keychain can get credentials
			keychainCmd := exec.Command("docker-credential-osxkeychain", "get")
			keychainCmd.Stdin = strings.NewReader(registry)
			keychainOutput, keychainErr := keychainCmd.CombinedOutput()
			fmt.Printf("Keychain get result: error=%v, output=%s\n", keychainErr, string(keychainOutput))
			if keychainErr != nil {
				fmt.Printf("Keychain error: %v, output: %s\n", keychainErr, string(keychainOutput))
			}
			gomega.Expect(keychainErr).ShouldNot(gomega.HaveOccurred(), "Keychain should retrieve credentials")
			gomega.Expect(string(keychainOutput)).To(gomega.ContainSubstring("testUser"), "Keychain should contain username")
			fmt.Printf("✓ Credentials stored in keychain\n")

			// 5. Push test
			ginkgo.By("Testing push with credentials")
			pushResult := command.New(o, "push", testImageTag).WithoutCheckingExitCode().Run()
			if pushResult.ExitCode() != 0 {
				fmt.Printf("Push failed: exit=%d, stderr=%s\n", pushResult.ExitCode(), string(pushResult.Err.Contents()))
			}
			gomega.Expect(pushResult.ExitCode()).To(gomega.Equal(0), "Push should succeed with credentials")
			fmt.Printf("✓ Push successful\n")

			// 6. Clean + Pull test
			ginkgo.By("Testing pull with credentials")
			command.Run(o, "system", "prune", "-f", "-a")
			pullResult := command.New(o, "pull", testImageTag).WithoutCheckingExitCode().Run()
			if pullResult.ExitCode() != 0 {
				fmt.Printf("Pull failed: exit=%d, stderr=%s\n", pullResult.ExitCode(), string(pullResult.Err.Contents()))
			}
			gomega.Expect(pullResult.ExitCode()).To(gomega.Equal(0), "Pull should succeed with credentials")
			fmt.Printf("✓ Pull successful\n")

			// 7. Clean + Run test (regular and detached)
			ginkgo.By("Testing run with credentials")
			command.Run(o, "system", "prune", "-f", "-a")
			runResult := command.New(o, "run", "--rm", testImageTag, "echo", "test-run").WithoutCheckingExitCode().Run()
			if runResult.ExitCode() != 0 {
				fmt.Printf("Run failed: exit=%d, stderr=%s\n", runResult.ExitCode(), string(runResult.Err.Contents()))
			}
			gomega.Expect(runResult.ExitCode()).To(gomega.Equal(0), "Run should succeed with credentials")
			fmt.Printf("✓ Run successful\n")

			command.Run(o, "system", "prune", "-f", "-a")
			detachedResult := command.New(o, "run", "-d", testImageTag, "sleep", "5").WithoutCheckingExitCode().Run()
			if detachedResult.ExitCode() != 0 {
				fmt.Printf("Detached run failed: exit=%d, stderr=%s\n", detachedResult.ExitCode(), string(detachedResult.Err.Contents()))
			}
			gomega.Expect(detachedResult.ExitCode()).To(gomega.Equal(0), "Detached run should succeed with credentials")
			containerID = strings.TrimSpace(string(detachedResult.Out.Contents()))
			command.Run(o, "rm", "-f", containerID)
			fmt.Printf("✓ Detached run successful\n")

			// 7b. Clean + Create test
			ginkgo.By("Testing create with credentials")
			command.Run(o, "system", "prune", "-f", "-a")
			createResult := command.New(o, "create", testImageTag, "echo", "test-create").WithoutCheckingExitCode().Run()
			if createResult.ExitCode() != 0 {
				fmt.Printf("Create failed: exit=%d, stderr=%s\n", createResult.ExitCode(), string(createResult.Err.Contents()))
			}
			gomega.Expect(createResult.ExitCode()).To(gomega.Equal(0), "Create should succeed with credentials")
			containerID = strings.TrimSpace(string(createResult.Out.Contents()))
			command.Run(o, "rm", containerID)
			fmt.Printf("✓ Create successful\n")

			// 8. Clean + Build test
			ginkgo.By("Testing build with private base image")
			command.Run(o, "system", "prune", "-f", "-a")
			dockerfileContent := fmt.Sprintf("FROM %s\nRUN echo 'test build'", testImageTag)
			buildDir := ffs.CreateTempDir("build-test")
			ginkgo.DeferCleanup(os.RemoveAll, buildDir)
			err = os.WriteFile(filepath.Join(buildDir, "Dockerfile"), []byte(dockerfileContent), 0644)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			buildResult := command.New(o, "build", "-t", "test-build", buildDir).WithoutCheckingExitCode().Run()
			if buildResult.ExitCode() != 0 {
				fmt.Printf("Build failed: exit=%d, stderr=%s\n", buildResult.ExitCode(), string(buildResult.Err.Contents()))
			}
			gomega.Expect(buildResult.ExitCode()).To(gomega.Equal(0), "Build should succeed with credentials")
			fmt.Printf("✓ Build successful\n")

			// 9. Logout
			ginkgo.By("Logging out from registry")
			command.Run(o, "logout", registry)

			// 10. Post-logout verification
			ginkgo.By("Verifying post-logout credential removal")
			configContent, err = os.ReadFile(filepath.Clean(configPath))
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			fmt.Printf("Config after logout: %s\n", string(configContent))
			gomega.Expect(string(configContent)).ToNot(gomega.ContainSubstring(registry), "Registry should be removed from auths after logout")

			// Verify keychain cannot get credentials
			keychainCmd = exec.Command("docker-credential-osxkeychain", "get")
			keychainCmd.Stdin = strings.NewReader(registry)
			keychainOutput, keychainErr = keychainCmd.CombinedOutput()
			fmt.Printf("Keychain get after logout: error=%v, output=%s\n", keychainErr, string(keychainOutput))
			if keychainErr == nil {
				fmt.Printf("WARNING: Keychain still has credentials after logout\n")
			}
			gomega.Expect(keychainErr).Should(gomega.HaveOccurred(), "Keychain should not retrieve credentials after logout")
			fmt.Printf("✓ Credentials removed from keychain\n")

			// Verify registry blocks unauthenticated access
			ginkgo.By("Verifying registry blocks unauthenticated access")
			command.Run(o, "system", "prune", "-f", "-a")
			unauthPullResult := command.New(o, "pull", testImageTag).WithoutCheckingExitCode().Run()
			fmt.Printf("Unauthenticated pull: exit=%d, stderr=%s\n", unauthPullResult.ExitCode(), string(unauthPullResult.Err.Contents()))
			gomega.Expect(unauthPullResult.ExitCode()).ToNot(gomega.Equal(0), "Registry should block unauthenticated pull")
			fmt.Printf("✓ Registry properly blocks unauthenticated access\n")
		})
	})
}
