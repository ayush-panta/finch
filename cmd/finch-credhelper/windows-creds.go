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
	socketPath := filepath.Join(homeDir, ".finch", "native-creds.sock")
	os.Remove(socketPath)

	// Listen on Unix socket
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}
	defer listener.Close()

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection")
			continue
		}

		// Handle each connection
		go func(c net.Conn) {
			defer c.Close()
				if err := processCredentialRequest(c); err != nil {
				log.Printf("Error processing credential request")
			}
		}(conn)
	}
}

func main() {
	if err := startWindowsCredentialServer(); err != nil {
		log.Printf("Windows credential server failed")
		os.Exit(1)
	}
}