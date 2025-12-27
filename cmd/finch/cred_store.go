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

	"github.com/containerd/nerdctl/v2/pkg/imgutil/dockerconfigresolver"
)

// customCredStore implements credential storage using native OS helpers
type customCredStore struct{}

func newCustomCredStore() *customCredStore {
	return &customCredStore{}
}

func (c *customCredStore) Store(registryURL *dockerconfigresolver.RegistryURL, credentials *dockerconfigresolver.Credentials) error {
	helperPath, err := c.getHelperPath()
	if err != nil {
		return err
	}

	cred := map[string]string{
		"ServerURL": registryURL.Host,
		"Username":  credentials.Username,
		"Secret":    credentials.Password,
	}

	credJSON, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	cmd := exec.Command(helperPath, "store")
	cmd.Stdin = strings.NewReader(string(credJSON))
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("credential helper store failed: %w", err)
	}

	return nil
}

func (c *customCredStore) Retrieve(registryURL *dockerconfigresolver.RegistryURL, _ bool) (*dockerconfigresolver.Credentials, error) {
	helperPath, err := c.getHelperPath()
	if err != nil {
		return &dockerconfigresolver.Credentials{}, err
	}

	cmd := exec.Command(helperPath, "get")
	cmd.Stdin = strings.NewReader(registryURL.Host)
	
	output, err := cmd.Output()
	if err != nil {
		return &dockerconfigresolver.Credentials{}, nil // Return empty creds if not found
	}

	var cred map[string]string
	if err := json.Unmarshal(output, &cred); err != nil {
		return &dockerconfigresolver.Credentials{}, err
	}

	return &dockerconfigresolver.Credentials{
		Username: cred["Username"],
		Password: cred["Secret"],
	}, nil
}

func (c *customCredStore) Erase(registryURL *dockerconfigresolver.RegistryURL) (map[string]error, error) {
	helperPath, err := c.getHelperPath()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(helperPath, "erase")
	cmd.Stdin = strings.NewReader(registryURL.Host)
	
	err = cmd.Run()
	errs := make(map[string]error)
	if err != nil {
		errs[registryURL.Host] = err
	}

	return errs, nil
}

func (c *customCredStore) FileStorageLocation(_ *dockerconfigresolver.RegistryURL) string {
	return "" // Native credential store, no file location
}

func (c *customCredStore) ShellCompletion() []string {
	return []string{} // Not implemented
}

func (c *customCredStore) getHelperPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	var helperName string
	switch runtime.GOOS {
	case "darwin":
		helperName = "docker-credential-osxkeychain"
	case "windows":
		helperName = "docker-credential-wincred.exe"
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	helperPath := filepath.Join(cwd, "_output", "cred-helpers", helperName)
	
	if _, err := os.Stat(helperPath); os.IsNotExist(err) {
		return "", fmt.Errorf("credential helper not found: %s", helperPath)
	}

	return helperPath, nil
}