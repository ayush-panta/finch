// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package credhelper for integrating credential helpers into Finch
package credhelper

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"

	"github.com/runfinch/finch/pkg/command"
	"github.com/runfinch/finch/pkg/config"
	"github.com/runfinch/finch/pkg/dependency"
	"github.com/runfinch/finch/pkg/flog"
	"github.com/runfinch/finch/pkg/path"
)

const (
	description = "Installing Credential Helper"
	errMsg      = "Failed to finish installing credential helper"
)

// NewDependencyGroup returns a dependency group that contains all the dependencies required to make credhelper work.
func NewDependencyGroup(
	execCmdCreator command.Creator,
	fs afero.Fs,
	fp path.Finch,
	logger flog.Logger,
	fc *config.Finch,
	finchDir string,
	arch string,
) *dependency.Group {
	deps := newDeps(execCmdCreator, fs, fp, logger, fc, finchDir, arch)
	return dependency.NewGroup(deps, description, errMsg)
}

type helperConfig struct {
	binaryName    string
	credHelperURL string
	hash          string
	installFolder string
	finchPath     string
}

func newDeps(
	execCmdCreator command.Creator,
	fs afero.Fs,
	fp path.Finch,
	logger flog.Logger,
	fc *config.Finch,
	finchDir string,
	arch string,
) []dependency.Dependency {
	var deps []dependency.Dependency
	empty := dependency.Dependency(nil)
	if fc == nil {
		deps = append(deps, empty)
		return deps
	}
	if fc.CredsHelpers == nil {
		deps = append(deps, empty)
		return deps
	}
	configs := map[string]helperConfig{}
	installFolder := filepath.Join(finchDir, "cred-helpers")

	// ECR Login helper
	const versionEcr = "0.9.0"
	const hashARM64 = "sha256:76aa3bb223d4e64dd4456376334273f27830c8d818efe278ab6ea81cb0844420"
	const hashAMD64 = "sha256:dd6bd933e439ddb33b9f005ad5575705a243d4e1e3d286b6c82928bcb70e949a"
	credHelperURLEcr := fmt.Sprintf("https://amazon-ecr-credential-helper-releases.s3.us-east-2.amazonaws.com"+
		"/%s/linux-%s/docker-credential-ecr-login", versionEcr, arch)

	hcEcr := helperConfig{
		binaryName:    "docker-credential-ecr-login",
		credHelperURL: credHelperURLEcr,
		installFolder: installFolder,
		finchPath:     finchDir,
	}

	if arch == "arm64" {
		hcEcr.hash = hashARM64
	} else {
		hcEcr.hash = hashAMD64
	}
	configs["ecr-login"] = hcEcr

	// Native credential helpers (osxkeychain, wincred)
	addNativeCredHelpers(configs, installFolder, finchDir, arch)

	for _, helper := range fc.CredsHelpers {
		if configs[helper] != (helperConfig{}) {
			binaries := newCredHelperBinary(fp, fs, execCmdCreator, logger, helper, configs[helper])
			deps = append(deps, dependency.Dependency(binaries))
		}
	}

	return deps
}

// addNativeCredHelpers adds osxkeychain and wincred helpers using finch-core config
func addNativeCredHelpers(configs map[string]helperConfig, installFolder, finchPath, arch string) {
	// macOS osxkeychain helper
	if arch == "arm64" {
		configs["osxkeychain"] = helperConfig{
			binaryName:    "docker-credential-osxkeychain",
			credHelperURL: "https://github.com/docker/docker-credential-helpers/releases/download/v0.9.4/docker-credential-osxkeychain-v0.9.4.darwin-arm64",
			hash:          "sha256:8db5b7cbcbe0870276e56aa416416161785e450708af64cda0f1be4c392dc2e5",
			installFolder: installFolder,
			finchPath:     finchPath,
		}
	} else {
		configs["osxkeychain"] = helperConfig{
			binaryName:    "docker-credential-osxkeychain",
			credHelperURL: "https://github.com/docker/docker-credential-helpers/releases/download/v0.9.4/docker-credential-osxkeychain-v0.9.4.darwin-amd64",
			hash:          "sha256:ad76d1a1e03def49edfa57fdb2874adf2c468cfa0438aae1b2589434796f7c01",
			installFolder: installFolder,
			finchPath:     finchPath,
		}
	}

	// Windows wincred helper
	configs["wincred"] = helperConfig{
		binaryName:    "docker-credential-wincred.exe",
		credHelperURL: "https://github.com/docker/docker-credential-helpers/releases/download/v0.9.4/docker-credential-wincred-v0.9.4.windows-amd64.exe",
		hash:          "sha256:66fdf4b50c83aeb04a9ea04af960abaf1a7b739ab263115f956b98bb0d16aa7e",
		installFolder: installFolder,
		finchPath:     finchPath,
	}
}
