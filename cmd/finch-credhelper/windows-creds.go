//go:build windows

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
)

// Windows socket server (for WSL2 socket forwarding)
func startWindowsCredentialServer() error {
	// Create socket path in user's Documents/finch-creds directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Setup file logging
	logPath := filepath.Join(homeDir, "Documents", "finch-creds", "finch", "_output", "finch-credhelper", "cred-bridge.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	socketPath := filepath.Join(homeDir, "Documents", "finch-creds", "finch", "_output", "finch-credhelper", "native-creds.sock")
	log.Printf("Starting Windows credential server on socket: %s", socketPath)

	// Remove existing socket if it exists
	os.Remove(socketPath)

	// Listen on Unix socket
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}
	defer listener.Close()

	log.Printf("Windows credential server listening...")

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		// Handle each connection
		go func(c net.Conn) {
			defer c.Close()
			if err := processCredentialRequest(c); err != nil {
				log.Printf("Error processing credential request: %v", err)
			}
		}(conn)
	}
}

func windowsKeychainHandler() {
	if err := startWindowsCredentialServer(); err != nil {
		log.Printf("Windows credential server failed: %v", err)
		os.Exit(1)
	}
}

func main() {
	windowsKeychainHandler()
}