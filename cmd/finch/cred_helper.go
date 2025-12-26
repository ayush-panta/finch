//go:build darwin || windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Docker credential helper protocol
type dockerCredential struct {
	ServerURL string `json:"ServerURL"`
	Username  string `json:"Username"`
	Secret    string `json:"Secret"`
}

// callNativeCredHelper calls the native credential helper
func callNativeCredHelper(action, serverURL, username, password string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	var helperName string
	switch runtime.GOOS {
	case "darwin":
		helperName = "docker-credential-osxkeychain"
	case "windows":
		helperName = "docker-credential-wincred.exe"
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	helperPath := filepath.Join(homeDir, ".finch", "cred-helpers", helperName)

	if _, err := os.Stat(helperPath); os.IsNotExist(err) {
		return fmt.Errorf("credential helper not found: %s", helperPath)
	}

	cmd := exec.Command(helperPath, action)

	if action == "store" {
		cred := dockerCredential{
			ServerURL: serverURL,
			Username:  username,
			Secret:    password,
		}

		credJSON, err := json.Marshal(cred)
		if err != nil {
			return fmt.Errorf("failed to marshal credentials: %w", err)
		}

		cmd.Stdin = strings.NewReader(string(credJSON))
	} else if action == "get" || action == "erase" {
		cmd.Stdin = strings.NewReader(serverURL)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("credential helper failed: %w - %s", err, string(output))
	}

	return nil
}

// callNativeCredHelperWithOutput calls the native credential helper and returns credentials
func callNativeCredHelperWithOutput(action, serverURL string) (*dockerCredential, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	var helperName string
	switch runtime.GOOS {
	case "darwin":
		helperName = "docker-credential-osxkeychain"
	case "windows":
		helperName = "docker-credential-wincred.exe"
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	helperPath := filepath.Join(homeDir, ".finch", "cred-helpers", helperName)

	if _, err := os.Stat(helperPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("credential helper not found: %s", helperPath)
	}

	cmd := exec.Command(helperPath, action)
	cmd.Stdin = strings.NewReader(serverURL)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("credential helper failed: %w", err)
	}

	if action == "get" {
		var creds dockerCredential
		if err := json.Unmarshal(output, &creds); err != nil {
			return nil, fmt.Errorf("failed to parse credential response: %w", err)
		}
		return &creds, nil
	}

	return &dockerCredential{}, nil
}