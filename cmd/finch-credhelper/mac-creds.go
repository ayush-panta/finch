//go:build darwin

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker-credential-helpers/credentials"
)

// no return as calls back to socket
func handleCredstoreRequest() error {

	// connect to socket, defer close til after return
	conn, err := net.FileConn(os.Stdin)
	if err != nil {
		return fmt.Errorf("ERROR: failed to get connection from stdin: %w", err)
	}
	defer conn.Close()

	// read from buffer
	buffer := make([]byte, maxBufferSize)
	data, err := conn.Read(buffer)
	if err != nil {
		return fmt.Errorf("ERROR:	 read error: %w", err)
	}

	// parse request
	request := strings.TrimSpace(string(buffer[:data]))
	command, input, err := parseCredstoreRequest(request)
	if err != nil {
		return fmt.Errorf("ERROR: %w", err)
	}

	// forward and handle request
	response, err := forwardToCredHelper(command, input)
	if err != nil {
		log.Printf("Credential helper error: %v", err)
	}

	// hmm... some cred handling here.
	if strings.Contains(response, "credentials not found") {
		response = ""
		log.Printf("Credentials not found - returning empty response")
	} else {
		log.Printf("Response to VM: %q", response)
	}

	// write back to socket, and return err if exists
	_, writeErr := conn.Write([]byte(response))
	return writeErr
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