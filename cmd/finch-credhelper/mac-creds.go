//go:build darwin

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

// macOS socket activation handler
func handleCredstoreRequest() error {
	// connect to socket, defer close til after return
	conn, err := net.FileConn(os.Stdin)
	if err != nil {
		return fmt.Errorf("ERROR: failed to get connection from stdin: %w", err)
	}
	defer conn.Close()

	// use shared credential processing logic
	return processCredentialRequest(conn)
}

func darwinKeychainHandler() {
	// Test if stdin is a network connection (inetd style)
	if _, err := net.FileConn(os.Stdin); err == nil {
		if err := handleCredstoreRequest(); err != nil {
			os.Exit(1)
		}
	} else {
		os.Exit(0)
	}
}

func main() {
	darwinKeychainHandler()
}