package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker-credential-helpers/credentials"
)

const (
    MaxBufferSize = 4096
    CredHelpersDir = "cred-helpers"
    FinchConfigDir = ".finch"
)

var credentialHelperNames = map[string]string{
    "darwin":  "docker-credential-osxkeychain",
    "windows": "docker-credential-wincred.exe",
}

// requests come in as "{command}\n{json}"
func parseCredstoreRequest(request string) (command, input string, err error) {
	lines := strings.Split(strings.TrimSpace(request), "\n")
	if len(lines) == 0 {
		return "", "", fmt.Errorf("empty request")
	}

	command = strings.TrimSpace(lines[0])
	if command == "list" {
		return command, "", nil
	}
	if len(lines) < 2 {
		return "", "", fmt.Errorf("command %s requires input", command)
	}

	return command, strings.TrimSpace(lines[1]), nil
}

// invokes the platform-specific credential helper
func executeCredentialHelper(command, input string) (string, error) {
	credHelperPath, err := getCredentialHelperPath()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(credHelperPath, command)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	response := strings.TrimSpace(string(output))

	if err != nil {
		if command == "get" {
			// Return empty credentials for failed get operations
			emptyCreds := credentials.Credentials{ServerURL: input, Username: "", Secret: ""}
			credsJSON, _ := json.Marshal(emptyCreds)
			return string(credsJSON), nil
		}
		return "", fmt.Errorf("credential helper failed: %w", err)
	}

	return response, nil
}

// get the name of the credhelper...
func getCredentialHelperPath() (string, error) {
	var helperName string
	switch runtime.GOOS {
	case "darwin":
		helperName = "docker-credential-osxkeychain"
	case "windows":
		helperName = "docker-credential-wincred.exe"
	default:
		return "", fmt.Errorf("credential helper not supported on %s", runtime.GOOS)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	path := filepath.Join(homeDir, ".finch", "cred-helpers", helperName)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("credential helper %s not found at %s", helperName, path)
	}
	return path, nil
}

// processCredentialRequest handles credential requests from the VM
func processCredentialRequest(conn interface{ Read([]byte) (int, error); Write([]byte) (int, error) }) error {
	const maxBufferSize = 4096
	
	buffer := make([]byte, maxBufferSize)
	n, err := conn.Read(buffer)
	if err != nil {
		return fmt.Errorf("failed to read request: %w", err)
	}

	request := strings.TrimSpace(string(buffer[:n]))
	command, input, err := parseCredstoreRequest(request)
	if err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}

	response, err := executeCredentialHelper(command, input)
	if err != nil {
		// For failed operations, return empty response instead of error
		response = ""
	}

	// Handle credential not found cases
	if strings.Contains(response, "credentials not found") {
		response = ""
	}

	_, err = conn.Write([]byte(response))
	return err
}
