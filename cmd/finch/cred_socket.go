//go:build darwin || windows

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	credSocketMutex sync.Mutex
	credSocketConn  net.Listener
)

// startCredSocket starts the credential socket for nerdctl commands
func startCredSocket(finchRootPath string) error {
	credSocketMutex.Lock()
	defer credSocketMutex.Unlock()

	// Skip if already running
	if credSocketConn != nil {
		return nil
	}

	// Create socket path - must match Lima's {{.Dir}}/sock/creds.sock expectation
	socketPath := filepath.Join(finchRootPath, "lima", "data", "finch", "sock", "creds.sock")
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Remove existing socket file
	os.Remove(socketPath)

	// Create Unix socket
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create credential socket: %w", err)
	}

	credSocketConn = listener
	
	// Start accepting connections in background
	go handleCredConnections(listener)
	
	return nil
}

// stopCredSocket stops the credential socket
func stopCredSocket() {
	credSocketMutex.Lock()
	defer credSocketMutex.Unlock()

	if credSocketConn != nil {
		credSocketConn.Close()
		credSocketConn = nil
	}
}

// handleCredConnections handles incoming credential requests
func handleCredConnections(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return // Socket closed
		}
		
		go func(c net.Conn) {
			defer c.Close()
			handleCredRequest(c)
		}(conn)
	}
}

// handleCredRequest processes get credential requests from nerdctl
func handleCredRequest(conn net.Conn) {
	// Read command and server URL
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}
	command := strings.TrimSpace(scanner.Text())
	
	if !scanner.Scan() {
		return
	}
	serverURL := strings.TrimSpace(scanner.Text())
	
	// Call native credential helper and capture output
	creds, err := getNativeCredentials(command, serverURL)
	if err != nil {
		// Return empty credential JSON with serverURL populated
		emptyCred := dockerCredential{
			ServerURL: serverURL,
			Username:  "",
			Secret:    "",
		}
		credJSON, _ := json.Marshal(emptyCred)
		conn.Write(credJSON)
		return
	}
	
	credJSON, _ := json.Marshal(creds)
	conn.Write(credJSON)
}

// getNativeCredentials calls the native credential helper and returns parsed credentials
func getNativeCredentials(action, serverURL string) (*dockerCredential, error) {
	// Strip port from server URL for credential lookup (keychain doesn't store ports)
	cleanURL := serverURL
	if idx := strings.LastIndex(serverURL, ":"); idx != -1 {
		// Check if this is a port (numeric after colon)
		portPart := serverURL[idx+1:]
		if len(portPart) > 0 && portPart[0] >= '0' && portPart[0] <= '9' {
			cleanURL = serverURL[:idx]
		}
	}
	
	creds, err := callNativeCredHelperWithOutput(action, cleanURL)
	if err != nil {
		return nil, err
	}
	// Restore original serverURL in response
	creds.ServerURL = serverURL
	return creds, nil
}

// withCredSocket wraps command execution with credential socket lifecycle
func withCredSocket(finchRootPath string, fn func() error) error {
	// Start socket
	if err := startCredSocket(finchRootPath); err != nil {
		return err
	}
	
	// Ensure cleanup
	defer stopCredSocket()
	
	// Execute command (nc.ncc.Create(runArgs...).Run())
	return fn()
}