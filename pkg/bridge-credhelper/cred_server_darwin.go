//go:build darwin

// Package bridgecredhelper provides credential server functionality for Finch.
package bridgecredhelper

import (
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
	fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Starting credential server with finchRootPath: %s\n", finchRootPath)
	globalCredServer.mu.Lock()
	defer globalCredServer.mu.Unlock()

	// Already running
	if globalCredServer.listener != nil {
		fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Server already running\n")
		return nil
	}

	socketPath := filepath.Join(finchRootPath, "lima", "data", "finch", "sock", "creds.sock")
	fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Creating socket at: %s\n", socketPath)
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

	fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Credential server started successfully, listening on %s\n", socketPath)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] handleConnections panicked: %v\n", r)
			}
			fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] handleConnections goroutine exiting\n")
		}()
		// Heartbeat to prove goroutine is alive
		go func() {
			for {
				select {
				case <-globalCredServer.ctx.Done():
					return
				case <-time.After(5 * time.Second):
					fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Server heartbeat - still alive\n")
				}
			}
		}()
		globalCredServer.handleConnections()
	}() // Accept connections in background
	return nil
}

// StopCredentialServer stops the credential server
func StopCredentialServer() {
	fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] StopCredentialServer called\n")
	globalCredServer.mu.Lock()
	defer globalCredServer.mu.Unlock()

	if globalCredServer.cancel != nil {
		fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Cancelling context\n")
		globalCredServer.cancel()
	}
	if globalCredServer.listener != nil {
		fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Closing listener\n")
		_ = globalCredServer.listener.Close()
		if globalCredServer.socketPath != "" {
			fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Removing socket file: %s\n", globalCredServer.socketPath)
			err := os.Remove(globalCredServer.socketPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Failed to remove socket: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Socket removed successfully\n")
			}
		}
		globalCredServer.listener = nil
		globalCredServer.socketPath = ""
	}
	fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] StopCredentialServer completed\n")
}

func (cs *credentialServer) handleConnections() {
	fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Starting to handle connections\n")
	for {
		select {
		case <-cs.ctx.Done():
			fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Context cancelled, stopping connection handler\n")
			return
		default:
			fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Context not cancelled, continuing...\n")
		}

		fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] About to call Accept()...\n")
		conn, err := cs.listener.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Accept failed: %v\n", err)
			return // Socket closed
		}
		fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Accepted connection from %s\n", conn.RemoteAddr())
		go func(c net.Conn) {
			defer func() { _ = c.Close() }()
			_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
			cs.handleRequest(c)
		}(conn)
		fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Connection handled, looping back...\n")
	}
}

func (cs *credentialServer) handleRequest(conn net.Conn) {
	// Read the entire request in one go
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Failed to read from connection: %v\n", err)
		_, _ = conn.Write([]byte("error: failed to read request"))
		return
	}

	// Split the request by newlines
	request := string(buffer[:n])
	lines := strings.Split(strings.TrimSpace(request), "\n")
	fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Received request: %q, lines: %v\n", request, lines)

	if len(lines) < 2 {
		fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Invalid request format, expected 2 lines, got %d\n", len(lines))
		_, _ = conn.Write([]byte("error: invalid request format"))
		return
	}

	command := strings.TrimSpace(lines[0])
	serverURL := strings.TrimSpace(lines[1])

	fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Parsed command: %q, serverURL: %q\n", command, serverURL)

	// Only handle GET operations
	if command != "get" {
		_, _ = conn.Write([]byte("error: only get operations supported"))
		return
	}

	// Call credential helper to get credentials from host keychain
	creds, err := callCredentialHelper("get", serverURL, "", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Credential helper failed: %v\n", err)
		// Return empty credentials if not found
		creds = &dockerCredential{ServerURL: serverURL}
	}

	// Return credentials as JSON
	credJSON, err := json.Marshal(creds)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Failed to marshal credentials: %v\n", err)
		_, _ = conn.Write([]byte("error: failed to marshal credentials"))
		return
	}
	fmt.Fprintf(os.Stderr, "[CREDSERVER DEBUG] Returning credentials: %s\n", string(credJSON))
	_, _ = conn.Write(credJSON)
}
