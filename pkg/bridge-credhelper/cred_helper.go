//go:build darwin

package bridgecredhelper

import (
	"encoding/base64"
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
	helperName, _ := credhelper.GetCredentialHelperForServer(serverURL, finchDir)

	// If no helper configured, return empty (for plaintext config)
	if helperName == "" {
		return "", fmt.Errorf("no credential helper configured")
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
		// No helper configured, try reading from config.json directly
		return readFromConfigFile(serverURL)
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

func readFromConfigFile(serverURL string) (*DockerCredential, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return &DockerCredential{ServerURL: serverURL}, nil
	}
	
	configPath := filepath.Join(homeDir, ".finch", "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &DockerCredential{ServerURL: serverURL}, nil
	}
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return &DockerCredential{ServerURL: serverURL}, nil
	}
	
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return &DockerCredential{ServerURL: serverURL}, nil
	}
	
	// Check auths section for credentials
	if auths, ok := config["auths"].(map[string]interface{}); ok {
		if auth, ok := auths[serverURL].(map[string]interface{}); ok {
			// Check for separate username/password fields
			if username, ok := auth["username"].(string); ok {
				if password, ok := auth["password"].(string); ok {
					return &DockerCredential{
						ServerURL: serverURL,
						Username:  username,
						Secret:    password,
					}, nil
				}
			}
			// Check for base64 encoded auth field
			if authStr, ok := auth["auth"].(string); ok {
				return decodeAuth(serverURL, authStr)
			}
		}
	}
	
	return &DockerCredential{ServerURL: serverURL}, nil
}

func decodeAuth(serverURL, authStr string) (*DockerCredential, error) {
	decoded, err := base64.StdEncoding.DecodeString(authStr)
	if err != nil {
		return &DockerCredential{ServerURL: serverURL}, nil
	}
	
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return &DockerCredential{ServerURL: serverURL}, nil
	}
	
	return &DockerCredential{
		ServerURL: serverURL,
		Username:  parts[0],
		Secret:    parts[1],
	}, nil
}
