package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker-credential-helpers/credentials"
)

const (
	maxBufferSize = 4096
)

func parseCredstoreRequest(request string) (command, input string, err error) {

	lines := strings.Split(request, "\n")
	if len(lines) == 0 {
		return "", "", fmt.Errorf("ERROR: Empty request.")
	}

	command = lines[0]
	if command == "list" { // too keep or not to keep?
		return command, "", nil
	}
	if len(lines) != 2 {
		return "", "", fmt.Errorf("ERROR: command %s requires input", command)
	}

	return command, lines[1], nil
}

func forwardToCredHelper(command, input string) (string, error) {
	log.Printf("Forwarding command: %s, input: %s", command, input)

	credHelperPath, err := getCredentialHelperPath()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(credHelperPath, command)
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	response := strings.TrimSpace(string(output))

	log.Printf("Raw output: %q", string(output))
	log.Printf("Error: %v", err)
	if cmd.ProcessState != nil {
		log.Printf("Exit code: %d", cmd.ProcessState.ExitCode())
	}

	if err != nil {
		log.Printf("Credential helper failed: %s", response)
		if command == "get" {
			emptyCreds := credentials.Credentials{ServerURL: input, Username: "", Secret: ""}
			credsJSON, _ := json.Marshal(emptyCreds)
			log.Printf("Returning empty credentials: %s", string(credsJSON))
			return string(credsJSON), nil
		}
		return response, err
	}

	log.Printf("Credential helper SUCCESS - response: %s", response)
	return response, nil
}

func getCredentialHelperPath() (string, error) {
	var helperName string
	switch runtime.GOOS {
	case "darwin":
		helperName = "docker-credential-osxkeychain"
	case "windows":
		helperName = "docker-credential-wincred.exe"
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS) // ?
	}

	path := filepath.Join(os.Getenv("HOME"), ".finch", "cred-helpers", helperName)
	_, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("ERROR: %s not found", helperName)
	}
	return path, nil
}

// Core credential processing logic - shared across platforms
func processCredentialRequest(conn interface{ Read([]byte) (int, error); Write([]byte) (int, error) }) error {
	// read from buffer
	buffer := make([]byte, maxBufferSize)
	data, err := conn.Read(buffer)
	if err != nil {
		return fmt.Errorf("ERROR: read error: %w", err)
	}

	// parse request
	request := strings.TrimSpace(string(buffer[:data]))
	command, input, err := parseCredstoreRequest(request)
	if err != nil {
		return fmt.Errorf("ERROR: %w", err)
	}

	// forward and handle request
	response, err := forwardToCredHelper(command, input)
	if err != nil {
		log.Printf("Credential helper error: %v", err)
	}

	// handle credential not found
	if strings.Contains(response, "credentials not found") {
		response = ""
		log.Printf("Credentials not found - returning empty response")
	} else {
		log.Printf("Response to VM: %q", response)
	}

	// write back to connection
	_, writeErr := conn.Write([]byte(response))
	return writeErr
}
