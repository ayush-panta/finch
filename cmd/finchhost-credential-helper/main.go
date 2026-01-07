//go:build !windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package main implements docker-credential-finchhost
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/docker/docker-credential-helpers/credentials"
)

// bufferSize is the buffer size for socket communication.
const bufferSize = 4096

// FinchHostCredentialHelper implements the credentials.Helper interface.
type FinchHostCredentialHelper struct{}

// Add is not implemented for Finch credential helper.
func (h FinchHostCredentialHelper) Add(*credentials.Credentials) error {
	return fmt.Errorf("not implemented")
}

// Delete is not implemented for Finch credential helper.
func (h FinchHostCredentialHelper) Delete(_ string) error {
	return fmt.Errorf("not implemented")
}

// List is not implemented for Finch credential helper.
func (h FinchHostCredentialHelper) List() (map[string]string, error) {
	return nil, fmt.Errorf("not implemented")
}

// Get retrieves credentials via socket to host.
func (h FinchHostCredentialHelper) Get(serverURL string) (string, string, error) {
	fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Get called for serverURL: %s\n", serverURL)

	finchDir := os.Getenv("FINCH_DIR")
	if finchDir == "" {
		fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] FINCH_DIR not set\n")
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] FINCH_DIR: %s\n", finchDir)

	var credentialSocketPath string
	// macOS: Use port-forwarded path
	credentialSocketPath = "/run/finch-user-sockets/creds.sock"
	fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Socket path: %s\n", credentialSocketPath)

	conn, err := net.Dial("unix", credentialSocketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Failed to connect to socket: %v\n", err)
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	defer func() { _ = conn.Close() }()
	fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Connected to socket successfully\n")

	serverURL = strings.ReplaceAll(serverURL, "\n", "")
	serverURL = strings.ReplaceAll(serverURL, "\r", "")

	request := "get\n" + serverURL + "\n"
	fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Sending request: %s", request)
	_, err = conn.Write([]byte(request))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Failed to write to socket: %v\n", err)
		return "", "", credentials.NewErrCredentialsNotFound()
	}

	response := make([]byte, bufferSize)
	n, err := conn.Read(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Failed to read from socket: %v\n", err)
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Received response (%d bytes): %s\n", n, string(response[:n]))

	var cred struct {
		ServerURL string `json:"ServerURL"`
		Username  string `json:"Username"`
		Secret    string `json:"Secret"`
	}
	if err := json.Unmarshal(response[:n], &cred); err != nil {
		fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Failed to unmarshal response: %v\n", err)
		return "", "", credentials.NewErrCredentialsNotFound()
	}

	if cred.Username == "" && cred.Secret == "" {
		fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Empty credentials returned\n")
		return "", "", credentials.NewErrCredentialsNotFound()
	}

	fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Successfully retrieved credentials for user: %s\n", cred.Username)
	return cred.Username, cred.Secret, nil
}

func main() {
	credentials.Serve(FinchHostCredentialHelper{})
}
