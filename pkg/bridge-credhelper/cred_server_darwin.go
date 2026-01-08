//go:build darwin

// Package bridgecredhelper provides credential server functionality for Finch.
package bridgecredhelper

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type credentialServer struct {
	mu         sync.Mutex
	listener   net.Listener
	socketPath string
	ctx        context.Context
	cancel     context.CancelFunc
}

var globalCredServer = &credentialServer{}

// testSocketConnectivity checks if socket is responsive
func testSocketConnectivity(socketPath string) error {
	conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// StartCredentialServer starts the credential server for VM lifecycle
func StartCredentialServer(finchRootPath string) error {
	globalCredServer.mu.Lock()
	defer globalCredServer.mu.Unlock()

	// Already running
	if globalCredServer.listener != nil {
		return nil
	}

	socketPath := filepath.Join(finchRootPath, "lima", "data", "finch", "sock", "creds.sock")
	if err := os.MkdirAll(filepath.Dir(socketPath), 0750); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Only remove if socket is stale (connection fails)
	if testSocketConnectivity(socketPath) != nil {
		_ = os.Remove(socketPath)
	} else {
		return fmt.Errorf("socket already in use: %s", socketPath)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create credential socket: %w", err)
	}

	// Set secure permissions on socket (owner-only access)
	if err := os.Chmod(socketPath, 0600); err != nil {
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	globalCredServer.listener = listener
	globalCredServer.socketPath = socketPath
	globalCredServer.ctx, globalCredServer.cancel = context.WithCancel(context.Background())

	go globalCredServer.handleConnections() // Accept connections in background
	return nil
}

// StopCredentialServer stops the credential server
func StopCredentialServer() {
	globalCredServer.mu.Lock()
	defer globalCredServer.mu.Unlock()

	if globalCredServer.cancel != nil {
		globalCredServer.cancel()
	}
	if globalCredServer.listener != nil {
		_ = globalCredServer.listener.Close()
		if globalCredServer.socketPath != "" {
			_ = os.Remove(globalCredServer.socketPath)
		}
		globalCredServer.listener = nil
		globalCredServer.socketPath = ""
	}
}

func (cs *credentialServer) handleConnections() {
	for {
		select {
		case <-cs.ctx.Done():
			return
		default:
		}
		
		conn, err := cs.listener.Accept()
		if err != nil {
			return // Socket closed
		}
		go func(c net.Conn) {
			defer func() { _ = c.Close() }()
			_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
			cs.handleRequest(c)
		}(conn)
	}
}

func (cs *credentialServer) handleRequest(conn net.Conn) {
	scanner := bufio.NewScanner(conn)

	// Read command (should be "get")
	if !scanner.Scan() {
		_, _ = conn.Write([]byte("error: failed to read command"))
		return
	}
	command := strings.TrimSpace(scanner.Text())

	// Read server URL
	if !scanner.Scan() {
		_, _ = conn.Write([]byte("error: failed to read server URL"))
		return
	}
	serverURL := strings.TrimSpace(scanner.Text())

	// Only handle GET operations
	if command != "get" {
		_, _ = conn.Write([]byte("error: only get operations supported"))
		return
	}

	// Call credential helper to get credentials from host keychain
	creds, err := callCredentialHelper("get", serverURL, "", "")
	if err != nil {
		// Return empty credentials if not found
		creds = &dockerCredential{ServerURL: serverURL}
	}

	// Return credentials as JSON
	credJSON, err := json.Marshal(creds)
	if err != nil {
		_, _ = conn.Write([]byte("error: failed to marshal credentials"))
		return
	}
	_, _ = conn.Write(credJSON)
}