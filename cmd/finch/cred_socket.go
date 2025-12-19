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
	fmt.Fprintf(os.Stderr, "[SOCKET] startCredSocket called with finchRootPath: %s\n", finchRootPath)
	credSocketMutex.Lock()
	defer credSocketMutex.Unlock()

	// Skip if already running
	if credSocketConn != nil {
		fmt.Fprintf(os.Stderr, "[SOCKET] Socket already running, skipping\n")
		return nil
	}

	// Create socket path - must match Lima's {{.Dir}}/sock/creds.sock expectation
	// Lima's {{.Dir}} points to the instance directory, not finchRootPath
	socketPath := filepath.Join(finchRootPath, "lima", "data", "finch", "sock", "creds.sock")
	fmt.Fprintf(os.Stderr, "[SOCKET] Creating socket at: %s\n", socketPath)
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "[SOCKET] Failed to create directory: %v\n", err)
		return fmt.Errorf("failed to create socket directory: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[SOCKET] Directory created successfully\n")

	// Remove existing socket file
	os.Remove(socketPath)

	// Create Unix socket
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[SOCKET] Failed to create socket: %v\n", err)
		return fmt.Errorf("failed to create credential socket: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[SOCKET] Socket created successfully at: %s\n", socketPath)
	
	// Verify socket file exists
	if _, err := os.Stat(socketPath); err != nil {
		fmt.Fprintf(os.Stderr, "[SOCKET] Warning: Socket file not found after creation: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "[SOCKET] Socket file verified at: %s\n", socketPath)
	}

	credSocketConn = listener
	
	// Start accepting connections in background
	fmt.Fprintf(os.Stderr, "[SOCKET] Starting background connection handler\n")
	go handleCredConnections(listener)
	
	fmt.Fprintf(os.Stderr, "[SOCKET] Socket setup complete and listening\n")
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
	fmt.Fprintf(os.Stderr, "[SOCKET] Starting connection handler\n")
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[SOCKET] Accept error: %v\n", err)
			return // Socket closed
		}
		
		fmt.Fprintf(os.Stderr, "[SOCKET] Accepted new connection\n")
		go func(c net.Conn) {
			defer func() {
				fmt.Fprintf(os.Stderr, "[SOCKET] Closing connection\n")
				c.Close()
			}()
			handleCredRequest(c)
		}(conn)
	}
}

// handleCredRequest processes get credential requests from nerdctl
func handleCredRequest(conn net.Conn) {
	fmt.Fprintf(os.Stderr, "[SOCKET] Handling credential request\n")
	
	// Read server URL
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		fmt.Fprintf(os.Stderr, "[SOCKET] Failed to read server URL\n")
		return
	}
	serverURL := strings.TrimSpace(scanner.Text())
	fmt.Fprintf(os.Stderr, "[SOCKET] Received request for server: %s\n", serverURL)
	
	// Call native credential helper and capture output
	creds, err := getNativeCredentials("get", serverURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[SOCKET] Failed to get credentials: %v\n", err)
		// Return empty credential JSON on error
		emptyCred := dockerCredential{
			ServerURL: "",
			Username:  "",
			Secret:    "",
		}
		credJSON, _ := json.Marshal(emptyCred)
		conn.Write(credJSON)
		return
	}
	
	fmt.Fprintf(os.Stderr, "[SOCKET] Sending credentials for %s\n", serverURL)
	credJSON, _ := json.Marshal(creds)
	conn.Write(credJSON)
}

// getNativeCredentials calls the native credential helper and returns parsed credentials
func getNativeCredentials(action, serverURL string) (*dockerCredential, error) {
	creds, err := callNativeCredHelperWithOutput(action, serverURL, "", "")
	if err != nil {
		return nil, err
	}
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
	
	// Execute command
	return fn()
}