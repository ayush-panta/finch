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
	"github.com/runfinch/common-tests/ffs"
	"github.com/runfinch/common-tests/fnet"
	"github.com/runfinch/common-tests/option"
)

// testNativeCredHelper tests native credential helper functionality.
var testNativeCredHelper = func(o *option.Option, installed bool) {
	ginkgo.Describe("Native Credential Helper", func() {
		ginkgo.It("should have finchhost credential helper in VM PATH", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			limaOpt, err := limaCtlOpt(installed)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			
			result := command.New(limaOpt, "shell", "finch", "command", "-v", "docker-credential-finchhost").WithoutCheckingExitCode().Run()
			gomega.Expect(result.ExitCode()).To(gomega.Equal(0), "docker-credential-finchhost should be in VM PATH")
		})

		ginkgo.It("should have native credential helper available on host", func() {
			var credHelper string
			if runtime.GOOS == "windows" {
				credHelper = "docker-credential-wincred"
			} else {
				credHelper = "docker-credential-osxkeychain"
			}

			// Check if native credential helper is available on the HOST system
			_, err := exec.LookPath(credHelper)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Native credential helper %s should be available on host", credHelper)
		})

		ginkgo.It("should work with registry push/pull workflow", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup authenticated registry using same technique as finch_config_file_remote_test.go
			filename := "htpasswd"
			registryImage := "public.ecr.aws/docker/library/registry:2"
			registryContainer := "auth-registry"
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

			// Test credential workflow: login, push, prune, pull
			command.Run(o, "login", registry, "-u", "testUser", "-p", "testPassword")
			command.New(o, "pull", "hello-world").WithTimeoutInSeconds(60).Run()
			command.New(o, "tag", "hello-world", registry+"/hello:test").Run()
			command.New(o, "push", registry+"/hello:test").WithTimeoutInSeconds(60).Run()
			command.New(o, "system", "prune", "-f", "-a").Run()
			command.New(o, "pull", registry+"/hello:test").WithTimeoutInSeconds(60).Run()
			command.New(o, "run", "--rm", registry+"/hello:test").WithTimeoutInSeconds(30).Run()

			// Test logout and verify credentials are removed from native store
			command.Run(o, "logout", registry)
			
			// Verify credentials no longer exist in native credential store by calling helper directly on HOST
			var credHelper string
			if runtime.GOOS == "windows" {
				credHelper = "docker-credential-wincred"
			} else {
				credHelper = "docker-credential-osxkeychain"
			}
			
			// Call credential helper directly on host system
			cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | %s get", registry, credHelper))
			err := cmd.Run()
			gomega.Expect(err).To(gomega.HaveOccurred(), "credentials should be removed from native store after logout")
		})
	})
}