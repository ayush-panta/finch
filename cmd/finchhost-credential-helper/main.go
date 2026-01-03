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

// Add is not implemented for Finch credential helper.
func (h FinchHostCredentialHelper) Add(*credentials.Credentials) error {
	return fmt.Errorf("not implemented")
}

// Delete is not implemented for Finch credential helper.
func (h FinchHostCredentialHelper) Delete(serverURL string) error {
	return fmt.Errorf("not implemented")
}

// List is not implemented for Finch credential helper.
func (h FinchHostCredentialHelper) List() (map[string]string, error) {
	return nil, fmt.Errorf("not implemented")
}

// Get retrieves credentials via socket to host.
func (h FinchHostCredentialHelper) Get(serverURL string) (string, string, error) {
	// Debug logging
	fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper: Get called for serverURL=%s\n", serverURL)
	
	finchDir := os.Getenv("FINCH_DIR")
	if finchDir == "" {
		fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper: FINCH_DIR not set\n")
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper: FINCH_DIR=%s\n", finchDir)

	var credentialSocketPath string
	if strings.Contains(os.Getenv("PATH"), "/mnt/c") || os.Getenv("WSL_DISTRO_NAME") != "" {
		credentialSocketPath = filepath.Join(finchDir, "lima", "data", "finch", "sock", "creds.sock")
	} else {
		credentialSocketPath = "/run/finch-user-sockets/creds.sock"
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper: socketPath=%s\n", credentialSocketPath)

	conn, err := net.Dial("unix", credentialSocketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper: Failed to connect to socket: %v\n", err)
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	defer conn.Close()

	serverURL = strings.ReplaceAll(serverURL, "\n", "")
	serverURL = strings.ReplaceAll(serverURL, "\r", "")

	request := "get\n" + serverURL + "\n"
	fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper: Sending request: %s\n", strings.TrimSpace(request))
	_, err = conn.Write([]byte(request))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper: Failed to write request: %v\n", err)
		return "", "", credentials.NewErrCredentialsNotFound()
	}

	response := make([]byte, BufferSize)
	n, err := conn.Read(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper: Failed to read response: %v\n", err)
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper: Received response: %s\n", string(response[:n]))

	var cred struct {
		ServerURL string `json:"ServerURL"`
		Username  string `json:"Username"`
		Secret    string `json:"Secret"`
	}
	if err := json.Unmarshal(response[:n], &cred); err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper: Failed to parse response: %v\n", err)
		return "", "", credentials.NewErrCredentialsNotFound()
	}

	if cred.Username == "" && cred.Secret == "" {
		fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper: Empty credentials returned\n")
		return "", "", credentials.NewErrCredentialsNotFound()
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] finchhost-credential-helper: Returning credentials for user: %s\n", cred.Username)
	return cred.Username, cred.Secret, nil
}

func main() {
	credentials.Serve(FinchHostCredentialHelper{})
}
