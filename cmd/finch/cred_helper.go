//go:build darwin || windows

package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/afero"

	"github.com/runfinch/finch/pkg/path"
	"github.com/runfinch/finch/pkg/system"
)

type dockerConfig struct {
	CredsStore string `json:"credsStore"`
}

type dockerCredential struct {
	ServerURL string `json:"ServerURL"`
	Username  string `json:"Username"`
	Secret    string `json:"Secret"`
}

func getHelperPath() (string, error) {
	// First try configured helpers from finch config
	if path, err := tryConfiguredCredentialHelpers(); err == nil {
		return path, nil
	}

	// Then try default OS credential helper in PATH
	if path, err := tryDefaultCredentialHelper(); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("no credential helper found")
}

func callCredentialHelper(action, serverURL, username, password string) (*dockerCredential, error) {
	helperPath, err := getHelperPath()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(helperPath, action)

	// Set input based on action
	if action == "store" {
		cred := dockerCredential{
			ServerURL: serverURL,
			Username:  username,
			Secret:    password,
		}
		credJSON, err := json.Marshal(cred)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal credentials: %w", err)
		}
		cmd.Stdin = strings.NewReader(string(credJSON))
	} else {
		cmd.Stdin = strings.NewReader(serverURL)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("credential helper failed: %w - %s", err, string(output))
	}

	// Parse output only for get
	if action == "get" {
		var creds dockerCredential
		if err := json.Unmarshal(output, &creds); err != nil {
			return nil, fmt.Errorf("failed to parse credential response: %w", err)
		}
		return &creds, nil
	}

	return nil, nil
}

func tryDefaultCredentialHelper() (string, error) {
	var helperName string
	switch runtime.GOOS {
	case "darwin":
		helperName = "docker-credential-osxkeychain"
	case "windows":
		helperName = "docker-credential-wincred.exe"
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	return exec.LookPath(helperName)
}

func tryConfiguredCredentialHelpers() (string, error) {
	configPath, err := getDockerConfigPath()
	if err != nil {
		return "", err
	}

	cfg, err := loadDockerConfig(configPath)
	if err != nil {
		return "", err
	}

	if cfg.CredsStore == "" {
		return "", fmt.Errorf("no credStore configured")
	}

	helperName := "docker-credential-" + cfg.CredsStore
	if runtime.GOOS == "windows" {
		helperName += ".exe"
	}

	// Look in system PATH first
	if path, err := exec.LookPath(helperName); err == nil {
		return path, nil
	}

	// Fall back to finch creds-helpers directory
	finchDir := filepath.Dir(filepath.Dir(configPath))
	credsHelperPath := filepath.Join(finchDir, "creds-helpers", helperName)
	if isValidBinary(credsHelperPath) {
		return credsHelperPath, nil
	}
	return "", fmt.Errorf("credential helper %s not found", helperName)
}

func getDockerConfigPath() (string, error) {
	stdLib := system.NewStdLib()
	home, err := stdLib.GetUserHome()
	if err != nil {
		return "", err
	}
	fp := path.Finch("")
	finchDir := fp.FinchDir(home)
	return filepath.Join(finchDir, "config.json"), nil
}

func loadDockerConfig(configPath string) (*dockerConfig, error) {
	fs := afero.NewOsFs()
	b, err := afero.ReadFile(fs, configPath)
	if err != nil {
		return nil, err
	}

	var cfg dockerConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func isValidBinary(path string) bool {
	info, err := afero.NewOsFs().Stat(path)
	return err == nil && info.Size() > 0
}