// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package main implements docker-credential-finchhost
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker-credential-helpers/credentials"
)

const BufferSize = 4096

// FinchHostCredentialHelper implements the credentials.Helper interface.
type FinchHostCredentialHelper struct{}

// Add stores credentials via socket to host.
func (h FinchHostCredentialHelper) Add(creds *credentials.Credentials) error {
	fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper Add called for: %s\n", creds.ServerURL)
	
	finchDir := os.Getenv("FINCH_DIR")
	if finchDir == "" {
		fmt.Fprintf(os.Stderr, "[DEBUG] FINCH_DIR not set\n")
		return fmt.Errorf("FINCH_DIR not set")
	}

	var credentialSocketPath string
	// Detect if running in WSL (Windows) or native Linux (macOS VM)
	if strings.Contains(os.Getenv("PATH"), "/mnt/c") || os.Getenv("WSL_DISTRO_NAME") != "" {
		// Windows WSL - use direct mount
		credentialSocketPath = filepath.Join(finchDir, "lima", "data", "finch", "sock", "creds.sock")
	} else {
		// macOS VM - use reverse port forwarded socket
		credentialSocketPath = "/run/finch-user-sockets/creds.sock"
	}

	// connect to socket
	conn, err := net.Dial("unix", credentialSocketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Socket connection failed: %v\n", err)
		return err
	}
	defer conn.Close()

	// send store command with credentials through socket
	message := fmt.Sprintf("store\n%s\n%s\n%s\n", creds.ServerURL, creds.Username, creds.Secret)
	fmt.Fprintf(os.Stderr, "[DEBUG] Sending store request\n")
	_, err = conn.Write([]byte(message))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Failed to write to socket: %v\n", err)
		return err
	}

	// read response
	response := make([]byte, BufferSize)
	n, err := conn.Read(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Failed to read from socket: %v\n", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] Store response: %s\n", string(response[:n]))

	// Check for error response
	if strings.HasPrefix(string(response[:n]), "error:") {
		return fmt.Errorf("credential store failed: %s", string(response[:n]))
	}

	return nil
}

// Delete removes credentials via socket to host.
func (h FinchHostCredentialHelper) Delete(serverURL string) error {
	fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper Delete called for: %s\n", serverURL)
	
	finchDir := os.Getenv("FINCH_DIR")
	if finchDir == "" {
		fmt.Fprintf(os.Stderr, "[DEBUG] FINCH_DIR not set\n")
		return fmt.Errorf("FINCH_DIR not set")
	}

	var credentialSocketPath string
	// Detect if running in WSL (Windows) or native Linux (macOS VM)
	if strings.Contains(os.Getenv("PATH"), "/mnt/c") || os.Getenv("WSL_DISTRO_NAME") != "" {
		// Windows WSL - use direct mount
		credentialSocketPath = filepath.Join(finchDir, "lima", "data", "finch", "sock", "creds.sock")
	} else {
		// macOS VM - use reverse port forwarded socket
		credentialSocketPath = "/run/finch-user-sockets/creds.sock"
	}

	// connect to socket
	conn, err := net.Dial("unix", credentialSocketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Socket connection failed: %v\n", err)
		return err
	}
	defer conn.Close()

	// send erase command through socket
	message := fmt.Sprintf("erase\n%s\n", serverURL)
	fmt.Fprintf(os.Stderr, "[DEBUG] Sending erase request\n")
	_, err = conn.Write([]byte(message))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Failed to write to socket: %v\n", err)
		return err
	}

	// read response
	response := make([]byte, BufferSize)
	n, err := conn.Read(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Failed to read from socket: %v\n", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] Erase response: %s\n", string(response[:n]))

	// Check for error response
	if strings.HasPrefix(string(response[:n]), "error:") {
		return fmt.Errorf("credential erase failed: %s", string(response[:n]))
	}

	return nil
}

// List is not implemented.
func (h FinchHostCredentialHelper) List() (map[string]string, error) {
	return nil, fmt.Errorf("not implemented")
}

// Get retrieves credentials via socket to host.
func (h FinchHostCredentialHelper) Get(serverURL string) (string, string, error) {
	// Debug output to stderr so it doesn't interfere with credential output
	fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper called for: %s\n", serverURL)
	
	finchDir := os.Getenv("FINCH_DIR")
	if finchDir == "" {
		fmt.Fprintf(os.Stderr, "[DEBUG] FINCH_DIR not set\n")
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] FINCH_DIR: %s\n", finchDir)

	var credentialSocketPath string
	// Detect if running in WSL (Windows) or native Linux (macOS VM)
	if strings.Contains(os.Getenv("PATH"), "/mnt/c") || os.Getenv("WSL_DISTRO_NAME") != "" {
		// Windows WSL - use direct mount
		credentialSocketPath = filepath.Join(finchDir, "lima", "data", "finch", "sock", "creds.sock")
	} else {
		// macOS VM - use reverse port forwarded socket
		credentialSocketPath = "/run/finch-user-sockets/creds.sock"
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] Socket path: %s\n", credentialSocketPath)

	// connect to socket
	conn, err := net.Dial("unix", credentialSocketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Socket connection failed: %v\n", err)
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	defer conn.Close()
	fmt.Fprintf(os.Stderr, "[DEBUG] Socket connected successfully\n")

	// sanitize server URL
	serverURL = strings.ReplaceAll(serverURL, "\n", "")
	serverURL = strings.ReplaceAll(serverURL, "\r", "")

	// send get command with URL through socket
	fmt.Fprintf(os.Stderr, "[DEBUG] Sending request: get\\n%s\\n\n", serverURL)
	_, err = conn.Write([]byte("get\n" + serverURL + "\n"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Failed to write to socket: %v\n", err)
		return "", "", credentials.NewErrCredentialsNotFound()
	}

	// read response
	response := make([]byte, BufferSize)
	n, err := conn.Read(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Failed to read from socket: %v\n", err)
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] Received response (%d bytes): %s\n", n, string(response[:n]))

	// parse response
	var cred struct {
		ServerURL string `json:"ServerURL"`
		Username  string `json:"Username"`
		Secret    string `json:"Secret"`
	}
	if err := json.Unmarshal(response[:n], &cred); err != nil {
		return "", "", credentials.NewErrCredentialsNotFound()
	}

	// Return empty credentials if no credentials found
	if cred.Username == "" && cred.Secret == "" {
		return "", "", credentials.NewErrCredentialsNotFound()
	}

	return cred.Username, cred.Secret, nil
}

func main() {
	credentials.Serve(FinchHostCredentialHelper{})
}