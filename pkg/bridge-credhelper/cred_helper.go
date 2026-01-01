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
	CredsStore  string            `json:"credsStore"`
	CredHelpers map[string]string `json:"credHelpers"`
}

type dockerCredential struct {
	ServerURL string `json:"ServerURL"`
	Username  string `json:"Username"`
	Secret    string `json:"Secret"`
}

func getHelperPath(serverURL string) (string, error) {
	// First try configured helper from config.json (handles both credHelpers and credsStore)
	if path, err := tryConfiguredCredentialHelpers(serverURL); err == nil {
		return path, nil
	}

	// Then try default OS credential helper in PATH
	if path, err := tryDefaultCredentialHelper(); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("no credential helper found - please install docker-credential-osxkeychain or configure a helper in ~/.finch/config.json")
}

func callCredentialHelper(action, serverURL, username, password string) (*dockerCredential, error) {
	helperPath, err := getHelperPath(serverURL)
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

func tryConfiguredCredentialHelpers(serverURL string) (string, error) {
	configPath, err := getDockerConfigPath()
	if err != nil {
		return "", err
	}

	cfg, err := loadDockerConfig(configPath)
	if err != nil {
		return "", err
	}

	// Extract registry hostname from serverURL
	registryHost := extractRegistryHost(serverURL)
	
	// First check credHelpers for registry-specific helper
	if cfg.CredHelpers != nil {
		if helperName, exists := cfg.CredHelpers[registryHost]; exists {
			fullHelperName := "docker-credential-" + helperName
			if runtime.GOOS == "windows" {
				fullHelperName += ".exe"
			}
			return exec.LookPath(fullHelperName)
		}
	}

	// Fallback to global credsStore
	if cfg.CredsStore == "" {
		return "", fmt.Errorf("no credStore configured")
	}

	helperName := "docker-credential-" + cfg.CredsStore
	if runtime.GOOS == "windows" {
		helperName += ".exe"
	}

	return exec.LookPath(helperName)
}

func extractRegistryHost(serverURL string) string {
	// Remove protocol if present
	host := strings.TrimPrefix(serverURL, "https://")
	host = strings.TrimPrefix(host, "http://")
	
	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}
	
	return host
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

