//go:build !windows

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package main implements docker-credential-finchhost
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/docker/docker-credential-helpers/credentials"
)

// FinchHostCredentialHelper implements the credentials.Helper interface.
type FinchHostCredentialHelper struct{}

type CredentialRequest struct {
	Action    string            `json:"action"`
	ServerURL string            `json:"serverURL"`
	Env       map[string]string `json:"env"`
}

type CredentialResponse struct {
	ServerURL string `json:"ServerURL"`
	Username  string `json:"Username"`
	Secret    string `json:"Secret"`
}

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

// Get retrieves credentials via HTTP to host.
func (h FinchHostCredentialHelper) Get(serverURL string) (string, string, error) {
	fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Get called for serverURL: %s\n", serverURL)

	// Collect credential-related environment variables (same as nerdctl_remote.go)
	credentialEnvs := []string{
		"COSIGN_PASSWORD", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN", "AWS_ECR_DISABLE_CACHE", "AWS_ECR_CACHE_DIR", "AWS_ECR_IGNORE_CREDS_STORAGE",
	}

	envMap := make(map[string]string)
	for _, key := range credentialEnvs {
		if val := os.Getenv(key); val != "" {
			envMap[key] = val
		}
	}
	fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Collected %d env vars\n", len(envMap))

	req := CredentialRequest{
		Action:    "get",
		ServerURL: strings.TrimSpace(serverURL),
		Env:       envMap,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Failed to marshal request: %v\n", err)
		return "", "", credentials.NewErrCredentialsNotFound()
	}

	// Create HTTP client with Unix socket transport
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(_, _ string) (net.Conn, error) {
				return net.Dial("unix", "/run/finch-user-sockets/creds.sock")
			},
		},
	}

	resp, err := client.Post("http://unix/credentials", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Failed to make HTTP request: %v\n", err)
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	defer resp.Body.Close()
	fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] HTTP response status: %s\n", resp.Status)

	var cred CredentialResponse
	if err := json.NewDecoder(resp.Body).Decode(&cred); err != nil {
		fmt.Fprintf(os.Stderr, "[FINCHHOST DEBUG] Failed to decode response: %v\n", err)
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
