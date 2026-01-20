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
	// Obtain relevant paths using base dir from where the process is started.
	socketPath := filepath.Join(finchRootPath, "lima", "data", "finch", "sock", "creds.sock")
	daemonPath := filepath.Join(finchRootPath, "finch-cred", "credserver")
	pidFile := filepath.Join(finchRootPath, "lima", "data", "finch", "cred-daemon.pid")

	// Proceed iff the process is not already started.
	if isDaemonRunning(pidFile) {
		return nil
	}

	// Construct command to start daemon as detached background process.
	// #nosec G204 -- daemonPath is constructed from finchRootPath, not user input
	cmd := exec.Command(daemonPath, socketPath)
	cmd.Stderr = nil
	cmd.Stdout = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Attempt to start the daemon.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start credential daemon: %w", err)
	}

	// Write the PID file with permissions to track the detached process.
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

// isDaemonRunning checks if the daemon process is alive by sending signal 0.
func isDaemonRunning(pidFile string) bool {
	process, err := getProcessFromPIDFile(pidFile)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

// stopDaemon attempts to gracefully stop the daemon with SIGTERM, waiting up to 2 seconds before force-killing.
func stopDaemon(pidFile string) error {
	defer func() { _ = os.Remove(pidFile) }()

	process, err := getProcessFromPIDFile(pidFile)
	if err != nil {
		return nil
	}

	// Ensure the process is still running.
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return nil
	}

	// Prompt the process to terminate.
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to terminate process: %w", err)
	}

	// Wait for process to terminate (up to 2 seconds).
	for i := 0; i < 20; i++ {
		if process.Signal(syscall.Signal(0)) != nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force kill if still running.
	_ = process.Signal(syscall.SIGKILL)
	return nil
}

// getProcessFromPIDFile reads the PID file and returns a handle to the process.
func getProcessFromPIDFile(pidFile string) (*os.Process, error) {
	// #nosec G304 -- pidFile path is constructed from finchRootPath, not user input
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, err
	}

	return os.FindProcess(pid)
}
