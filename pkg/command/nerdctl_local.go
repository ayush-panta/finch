/*
   Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
   SPDX-License-Identifier: Apache-2.0

   Portions of this code are derived from nerdctl:
   Copyright The containerd Authors.
   Licensed under the Apache License, Version 2.0.
*/

//go:build darwin || windows

package command

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
	"github.com/containerd/nerdctl/v2/pkg/imgutil/dockerconfigresolver"
)

// Nerdctl prompt errors
var (
	ErrUsernameIsRequired = errors.New("username is required")
	ErrPasswordIsRequired = errors.New("password is required")
	ErrReadingUsername    = errors.New("unable to read username")
	ErrReadingPassword    = errors.New("unable to read password")
	ErrNotATerminal       = errors.New("stdin is not a terminal (Hint: use `finch login --username=USERNAME --password-stdin`)")
)

// NerdctlLogin uses nerdctl's logic but with host credential helper
func NerdctlLogin(serverAddress, username, password string, stdout io.Writer) error {
	// Parse registry URL using nerdctl's logic
	registryURL, err := dockerconfigresolver.Parse(serverAddress)
	if err != nil {
		return err
	}

	// Create empty credentials struct
	credentials := &dockerconfigresolver.Credentials{}

	// Use nerdctl's prompt logic if credentials are missing
	err = promptUserForAuthentication(credentials, username, password, stdout)
	if err != nil {
		return err
	}

	// Store credentials using host credential helper
	err = callNativeCredHelper("store", registryURL.Host, credentials.Username, credentials.Password)
	if err != nil {
		return fmt.Errorf("error saving credentials: %w", err)
	}

	_, err = fmt.Fprintln(stdout, "Login Succeeded")
	return err
}

// NerdctlLogout uses nerdctl's logic but with host credential helper
func NerdctlLogout(serverAddress string, stdout io.Writer) error {
	// Parse registry URL using nerdctl's logic
	registryURL, err := dockerconfigresolver.Parse(serverAddress)
	if err != nil {
		return err
	}

	// Erase credentials using host credential helper
	err = callNativeCredHelper("erase", registryURL.Host, "", "")
	if err != nil {
		return fmt.Errorf("failed to erase credentials for: %s - %w", registryURL.Host, err)
	}

	_, err = fmt.Fprintf(stdout, "Removed login credentials for %s\n", registryURL.Host)
	return err
}

// promptUserForAuthentication - copied from nerdctl
func promptUserForAuthentication(credentials *dockerconfigresolver.Credentials, username, password string, stdout io.Writer) error {
	var err error

	if username = strings.TrimSpace(username); username == "" {
		username = credentials.Username
		if username == "" {
			_, _ = fmt.Fprint(stdout, "Enter Username: ")
			username, err = readUsername()
			if err != nil {
				return err
			}
			username = strings.TrimSpace(username)
			if username == "" {
				return ErrUsernameIsRequired
			}
		}
	}

	if password == "" {
		_, _ = fmt.Fprint(stdout, "Enter Password: ")
		password, err = readPassword()
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(stdout)
		password = strings.TrimSpace(password)
		if password == "" {
			return ErrPasswordIsRequired
		}
	}

	credentials.Username = username
	credentials.Password = password
	return nil
}

// readUsername - copied from nerdctl
func readUsername() (string, error) {
	fd := os.Stdin
	if !term.IsTerminal(int(fd.Fd())) {
		return "", ErrNotATerminal
	}
	username, err := bufio.NewReader(fd).ReadString('\n')
	if err != nil {
		return "", errors.Join(ErrReadingUsername, err)
	}
	return strings.TrimSpace(username), nil
}

// readPassword - copied from nerdctl
func readPassword() (string, error) {
	fd := syscall.Stdin
	if !term.IsTerminal(fd) {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return "", err
		}
		defer tty.Close()
		fd = int(tty.Fd())
	}
	bytePassword, err := term.ReadPassword(fd)
	if err != nil {
		return "", errors.Join(ErrReadingPassword, err)
	}
	return string(bytePassword), nil
}

// callNativeCredHelper calls the host's native credential helper
// This function needs to be implemented based on your existing credential helper logic
func callNativeCredHelper(action, serverURL, username, password string) error {
	// TODO: Implement this function to call your existing host credential helper
	// This should replace the previous callNativeCredHelper implementation
	return fmt.Errorf("callNativeCredHelper not implemented yet")
}