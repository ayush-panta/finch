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

type credentialSocket struct {
	mu       sync.Mutex
	listener net.Listener
}
var globalCredSocket = &credentialSocket{}

// open the socket that serves as bridge between vm and host
func (cs *credentialSocket) start(finchRootPath string) error {

	// prevent race condition on socket management
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// early break if socket already active
	if cs.listener != nil {
		return nil
	}

	socketPath := filepath.Join(finchRootPath, "lima", "data", "finch", "sock", "creds.sock")
	if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create credential socket: %w", err)
	}
	cs.listener = listener

	// routes connections through socket in separate threads
	go cs.handleConnections()
	return nil
}

// stop stops the credential socket
func (cs *credentialSocket) stop() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.listener != nil {
		cs.listener.Close()
		cs.listener = nil
	}
}

// handleConnections handles incoming credential requests
func (cs *credentialSocket) handleConnections() {
	for {
		conn, err := cs.listener.Accept()
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
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading command: %v\n", err)
		}
		return
	}
	command := strings.TrimSpace(scanner.Text())
	
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading server URL: %v\n", err)
		}
		return
	}
	serverURL := strings.TrimSpace(scanner.Text())
	
	creds, err := callNativeCredHelperWithOutput(command, serverURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Credential lookup failed for %s: %v\n", serverURL, err)
		// Return empty credential JSON with serverURL populated
		emptyCred := dockerCredential{
			ServerURL: serverURL,
			Username:  "",
			Secret:    "",
		}
		credJSON, err := json.Marshal(emptyCred)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling empty credentials: %v\n", err)
			return
		}
		if _, err := conn.Write(credJSON); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing empty credentials: %v\n", err)
		}
		return
	}
	
	credJSON, err := json.Marshal(creds)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling credentials: %v\n", err)
		return
	}
	if _, err := conn.Write(credJSON); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing credentials: %v\n", err)
	}
}

// withCredSocket wraps command execution with credential socket lifecycle
func withCredSocket(finchRootPath string, fn func() error) error {
	if err := globalCredSocket.start(finchRootPath); err != nil {
		return err
	}
	defer globalCredSocket.stop()
	return fn()
}