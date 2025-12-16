//go:build windows

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/Microsoft/go-winio"
)

// Windows socket server (for WSL2 socket forwarding)
func startWindowsCredentialServer() error {

	pipePath := filepath.Join(`\\.\pipe\native-creds`)
	os.Remove(pipePath)
	config := &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;CO)(A;;GA;;;SY)", // owner + "root" (need to verify)
		InputBufferSize:    4096,
		OutputBufferSize:   4096,
	}
	listener, err := winio.ListenPipe(pipePath, config)
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
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
