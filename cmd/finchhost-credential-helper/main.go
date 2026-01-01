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

// Add is not implemented.
func (h FinchHostCredentialHelper) Add(*credentials.Credentials) error {
	return fmt.Errorf("not implemented")
}

// Delete is not implemented.
func (h FinchHostCredentialHelper) Delete(serverURL string) error {
	return fmt.Errorf("not implemented")
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
	_, err = conn.Write([]byte("get\n" + serverURL + "\n"))
	if err != nil {
		return "", "", credentials.NewErrCredentialsNotFound()
	}

	// read response
	response := make([]byte, BufferSize)
	n, err := conn.Read(response)
	if err != nil {
		return "", "", credentials.NewErrCredentialsNotFound()
	}

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