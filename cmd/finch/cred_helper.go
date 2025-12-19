//go:build darwin || windows

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
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

// Native credential helper call
func callNativeCredHelper(action, serverURL, username, password string) error {
	// Get finch directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Determine credential helper binary name based on OS
	var helperName string
	switch runtime.GOOS {
	case "darwin":
		helperName = "docker-credential-osxkeychain"
	case "windows":
		helperName = "docker-credential-wincred.exe"
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	// Build path to credential helper
	helperPath := filepath.Join(homeDir, ".finch", "cred-helpers", helperName)

	// Check if helper exists
	if _, err := os.Stat(helperPath); os.IsNotExist(err) {
		return fmt.Errorf("credential helper not found: %s", helperPath)
	}

	// Create command
	cmd := exec.Command(helperPath, action)

	// For store action, send credential data via stdin
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
		// For get/erase actions, send server URL via stdin
		cmd.Stdin = strings.NewReader(serverURL)
	}

	// Run command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("credential helper failed: %w - %s", err, string(output))
	}

	return nil
}

// callNativeCredHelperWithOutput calls the native credential helper and returns the parsed credentials
func callNativeCredHelperWithOutput(action, serverURL, username, password string) (*dockerCredential, error) {
	// Get finch directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Determine credential helper binary name based on OS
	var helperName string
	switch runtime.GOOS {
	case "darwin":
		helperName = "docker-credential-osxkeychain"
	case "windows":
		helperName = "docker-credential-wincred.exe"
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	// Build path to credential helper
	helperPath := filepath.Join(homeDir, ".finch", "cred-helpers", helperName)

	// Check if helper exists
	if _, err := os.Stat(helperPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("credential helper not found: %s", helperPath)
	}

	// Create command
	cmd := exec.Command(helperPath, action)

	// For get action, send server URL via stdin
	if action == "get" {
		cmd.Stdin = strings.NewReader(serverURL)
	} else if action == "store" {
		// For store action, send credential data via stdin
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
	} else if action == "erase" {
		cmd.Stdin = strings.NewReader(serverURL)
	}

	// Run command and capture output
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("credential helper failed: %w", err)
	}

	// For get action, parse the JSON response
	if action == "get" {
		var creds dockerCredential
		if err := json.Unmarshal(output, &creds); err != nil {
			return nil, fmt.Errorf("failed to parse credential response: %w", err)
		}
		return &creds, nil
	}

	// For other actions, return empty credentials
	return &dockerCredential{}, nil
}

// Simplified version of nerdctl's dockerconfigresolver.Parse
func parseRegistryURL(serverAddress string) (string, error) {
	if serverAddress == "" {
		return "https://index.docker.io/v1/", nil
	}

	// Add https:// if no scheme
	if !strings.Contains(serverAddress, "://") {
		serverAddress = "https://" + serverAddress
	}

	u, err := url.Parse(serverAddress)
	if err != nil {
		return "", fmt.Errorf("invalid registry URL: %w", err)
	}

	return u.Host, nil
}