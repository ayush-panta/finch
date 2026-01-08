//go:build darwin

package bridgecredhelper

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/runfinch/finch/pkg/dependency/credhelper"
)

type DockerCredential struct {
	ServerURL string `json:"ServerURL"`
	Username  string `json:"Username"`
	Secret    string `json:"Secret"`
}

func getHelperPath(serverURL string) (string, error) {
	// Get finch directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return getDefaultHelperPath()
	}
	finchDir := filepath.Join(homeDir, ".finch")

	// Use existing credhelper package to get the right helper
	helperName, err := credhelper.GetCredentialHelperForServer(serverURL, finchDir)
	if err != nil {
		// Fall back to OS default if config reading fails
		return getDefaultHelperPath()
	}

	// Look up the binary with docker-credential- prefix
	return exec.LookPath("docker-credential-" + helperName)
}

// Allow to fall back to OS default for case when no credStore found (for robustness)
func getDefaultHelperPath() (string, error) {
	return exec.LookPath("docker-credential-osxkeychain")
}

func CallCredentialHelper(action, serverURL, username, password string) (*DockerCredential, error) {
	helperPath, err := getHelperPath(serverURL)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(helperPath, action) //nolint:gosec // helperPath is validated by exec.LookPath

	// Set input based on action
	if action == "store" {
		cred := DockerCredential{
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
		var creds DockerCredential
		if err := json.Unmarshal(output, &creds); err != nil {
			return nil, fmt.Errorf("failed to parse credential response: %w", err)
		}
		return &creds, nil
	}

	return nil, nil
}
