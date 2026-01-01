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
			hash:          "sha256:123e810d1705b17fb0195f7e640e5a2f5f04058a121041343d394d480027201a752c328ff1aa28fcdecbb3e544394e4f8edf0dac03b6c3845d9b1a0874514053",
			installFolder: installFolder,
			finchPath:     finchPath,
		}
	} else {
		configs["osxkeychain"] = helperConfig{
			binaryName:    "docker-credential-osxkeychain",
			credHelperURL: "https://github.com/docker/docker-credential-helpers/releases/download/v0.9.4/docker-credential-osxkeychain-v0.9.4.darwin-amd64",
			hash:          "sha256:21fe706c5044f990d333b065a8e99e80fdf63c8a8c3b3f8f178791a0412bf1d359878c906cc732d8fd8b69a5546a8dc3a3a6c8902a2f84609d0326f93c11a31f",
			installFolder: installFolder,
			finchPath:     finchPath,
		}
	}

	// Windows wincred helper
	configs["wincred"] = helperConfig{
		binaryName:    "docker-credential-wincred.exe",
		credHelperURL: "https://github.com/docker/docker-credential-helpers/releases/download/v0.9.4/docker-credential-wincred-v0.9.4.windows-amd64.exe",
		hash:          "sha256:ce9c9c2e3119e91edc2c20b564d5f8929b8052f3ed0b3c1e2bfcb74bffecd2f38b11d590ee6f8124bb21f40c6a959958a6d325648d9e06c6e256c05f8e3491a3",
		installFolder: installFolder,
		finchPath:     finchPath,
	}
}
