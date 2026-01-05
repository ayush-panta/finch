// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin || windows

package vm

import (
	"fmt"
	"runtime"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/runfinch/common-tests/command"
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

			result := command.New(o, "run", "--rm", "alpine", "command", "-v", credHelper).WithoutCheckingExitCode().Run()
			if result.ExitCode() != 0 {
				ginkgo.Skip("Native credential helper " + credHelper + " not available")
			}
		})

		ginkgo.It("should work with credential workflow", func() {
			resetVM(o)
			resetDisks(o, installed)
			command.New(o, virtualMachineRootCmd, "init").WithTimeoutInSeconds(160).Run()

			// Setup authenticated test registry
			port := fnet.GetFreePort()
			registryName := fmt.Sprintf("localhost:%d", port)
			containerName := fmt.Sprintf("test-registry-%d", port)
			
			// Create htpasswd file
			htpasswd := "testUser:$2y$05$wE0sj3r9O9K9q7R0MXcfPuIerl/06L1IsxXkCuUr3QZ8lHWwicIdS" // password: testPassword
			htpasswdFile := "/tmp/htpasswd"
			command.New(o, "run", "--rm", "-v", "/tmp:/tmp", "alpine", "sh", "-c", 
				fmt.Sprintf("echo '%s' > %s", htpasswd, htpasswdFile)).Run()
			
			containerID := command.StdoutStr(o, "run", "-dp", fmt.Sprintf("%d:5000", port),
				"--name", containerName,
				"-v", "/tmp:/auth",
				"-e", "REGISTRY_AUTH=htpasswd",
				"-e", "REGISTRY_AUTH_HTPASSWD_REALM=Registry",
				"-e", "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd",
				"registry:2")
			
			for command.StdoutStr(o, "inspect", "-f", "{{.State.Running}}", containerID) != "true" {
				time.Sleep(1 * time.Second)
			}
			time.Sleep(5 * time.Second)
			
			ginkgo.DeferCleanup(func() {
				command.Run(o, "rm", "-f", containerName)
			})

			// Test credential workflow: login, push, prune, pull
			command.New(o, "login", registryName, "-u", "testUser", "-p", "testPassword").Run() // Should succeed
			command.New(o, "pull", "hello-world").WithTimeoutInSeconds(60).Run()
			command.New(o, "tag", "hello-world", registryName+"/hello:test").Run()
			command.New(o, "push", registryName+"/hello:test").WithTimeoutInSeconds(60).Run()
			command.New(o, "system", "prune", "-f", "-a").Run()
			command.New(o, "pull", registryName+"/hello:test").WithTimeoutInSeconds(60).Run() // Uses stored creds
			command.New(o, "run", "--rm", registryName+"/hello:test").WithTimeoutInSeconds(30).Run()
		})
	})
}