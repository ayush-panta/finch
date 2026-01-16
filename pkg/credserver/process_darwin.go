// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

// Package credserver provides credential server operations for macOS.
package credserver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// StartCredentialServer starts the credential daemon process that handles requests from Lima VM via mounted socket.
// Called during finch vm init or start.
func StartCredentialServer(finchRootPath string) error {
	socketPath := filepath.Join(finchRootPath, "lima", "data", "finch", "sock", "creds.sock")
	daemonPath := filepath.Join(finchRootPath, "finch-cred", "credserver")
	pidFile := filepath.Join(finchRootPath, "lima", "data", "finch", "cred-daemon.pid")

	if isDaemonRunning(pidFile) {
		return nil
	}

	// #nosec G204 -- daemonPath is constructed from finchRootPath, not user input
	cmd := exec.Command(daemonPath, socketPath)
	cmd.Stderr = nil
	cmd.Stdout = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start credential daemon: %w", err)
	}

	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// StopCredentialServer stops the credential daemon process.
// Called during finch vm stop or remove.
func StopCredentialServer(finchRootPath string) error {
	pidFile := filepath.Join(finchRootPath, "lima", "data", "finch", "cred-daemon.pid")
	return stopDaemon(pidFile)
}

func isDaemonRunning(pidFile string) bool {
	// #nosec G304 -- pidFile path is constructed from finchRootPath, not user input
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func stopDaemon(pidFile string) error {
	// #nosec G304 -- pidFile path is constructed from finchRootPath, not user input
	data, err := os.ReadFile(pidFile)
	if err != nil {
		// PID file doesn't exist, nothing to stop
		return nil
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		// Invalid PID, clean up the file
		_ = os.Remove(pidFile)
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		// Process doesn't exist, clean up the file
		_ = os.Remove(pidFile)
		return nil
	}

	// Check if process is actually running
	if err := process.Signal(syscall.Signal(0)); err != nil {
		// Process not running, clean up the file
		_ = os.Remove(pidFile)
		return nil
	}

	// Send SIGTERM for graceful shutdown
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Failed to send signal, but clean up the file anyway
		_ = os.Remove(pidFile)
		return fmt.Errorf("failed to terminate process: %w", err)
	}

	// Wait for process to actually terminate (up to 2 seconds)
	for i := 0; i < 20; i++ {
		if err := process.Signal(syscall.Signal(0)); err != nil {
			// Process is dead
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force kill if still running
	if err := process.Signal(syscall.Signal(0)); err == nil {
		_ = process.Signal(syscall.SIGKILL)
		time.Sleep(100 * time.Millisecond)
	}

	// Remove PID file
	_ = os.Remove(pidFile)
	return nil
}


