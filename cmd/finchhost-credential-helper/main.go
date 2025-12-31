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
	"runtime"
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
	finchDir := os.Getenv("FINCH_DIR")
	if finchDir == "" {
		return "", "", credentials.NewErrCredentialsNotFound()
	}

	var credentialSocketPath string
	if runtime.GOOS == "darwin" {
		credentialSocketPath = "/run/finch-user-sockets/creds.sock"
	} else if runtime.GOOS == "windows" {
		credentialSocketPath = filepath.Join(finchDir, "lima", "data", "finch", "sock", "creds.sock")
	} else {
		return "", "", credentials.NewErrCredentialsNotFound()
	}

	// connect to socket
	conn, err := net.Dial("unix", credentialSocketPath)
	if err != nil {
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	defer conn.Close()

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