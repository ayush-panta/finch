//go:build darwin || windows

package bridgecredhelper

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/runfinch/finch/pkg/dependency/credhelper"
)

type dockerCredential struct {
	ServerURL string `json:"ServerURL"`
	Username  string `json:"Username"`
	Secret    string `json:"Secret"`
}

func getHelperPath(serverURL, finchRootPath string) (string, error) {
	// Try configured helper first
	helperName, err := credhelper.GetCredentialHelperForServer(serverURL, finchRootPath)
	if err == nil {
		if path, err := exec.LookPath("docker-credential-" + helperName); err == nil {
			return path, nil
		}
	}

	// Fall back to OS default
	return getDefaultHelperPath()
}

// Allow to fall back to OS default for unlikely case when no credStore found (for robustness)
func getDefaultHelperPath() (string, error) {
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

func callCredentialHelper(action, serverURL, username, password, finchRootPath string) (*dockerCredential, error) {
	helperPath, err := getHelperPath(serverURL, finchRootPath)
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
