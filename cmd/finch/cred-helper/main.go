// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

// const sockAddress = "/tmp/cred.sock"
const hostAddr = "0.0.0.0:8080"  // Listen on all interfaces

func main() {

	// start the socket
	fmt.Println("Credential helper daemon started")
	socket, err := net.Listen("tcp", hostAddr)
	if err != nil {
		log.Fatal(err)
	}

	// cleanup of socket as goroutine
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		// os.Remove(hostAddr)
		os.Exit(1)
	}()

	for {

		// accept the connection
		conn, err := socket.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			// create buffer for socket
			buf := make([]byte, 4096)
			n, err := conn.Read(buf)
			if err != nil {
				log.Printf("Read error: %v", err)
				return
			}

			// need to read in the data
			message := strings.TrimSpace(string(buf[:n]))
			lines := strings.Split(message, "\n")
			if len(lines) == 0 {
				conn.Write([]byte("Error: empty message"))
				return
			}

			// Parse command and data
			cmdName := strings.TrimSpace(lines[0])
			var stdinData string
			if len(lines) > 1 {
				// Decode HTML entities like &quot; -> "
				stdinData = strings.ReplaceAll(strings.Join(lines[1:], "\n"), "&quot;", "\"")
			}

			// Normalize Docker Hub URLs to canonical format
			if cmdName == "get" && stdinData != "" {
				if strings.Contains(stdinData, "docker.io") || strings.Contains(stdinData, "registry-1.docker.io") {
					stdinData = "https://index.docker.io/v1/"
				}
			}

			fmt.Printf("[RECV] Command: '%s', Data: '%s'\n", cmdName, stdinData)

			binaryPath := "/Users/ayushkp/Documents/finch-creds/finch/_output/bin/cred-helpers/docker-credential-osxkeychain"
			cmd := exec.Command(binaryPath, cmdName)
			if stdinData != "" {
				cmd.Stdin = strings.NewReader(stdinData)
			}

			// Capture both stdout and stderr
			output, err := cmd.CombinedOutput()
			if err != nil {
				// Check if it's a "credentials not found" error
				outputStr := string(output)
				if strings.Contains(outputStr, "credentials not found") || 
				   strings.Contains(outputStr, "could not be found in the keychain") ||
				   strings.Contains(outputStr, "not correct") {
					fmt.Printf("[SEND] No credentials found, allowing anonymous access\n")
					conn.Write([]byte("")) // Empty = no credentials, try anonymous
					return
				}
				fmt.Printf("[SEND] Error: %v, Output: '%s'\n", err, outputStr)
				conn.Write([]byte("")) // Return empty for any error to allow fallback
				return
			}

			fmt.Printf("[SEND] Success: '%s'\n", string(output))
			conn.Write(output)
		}(conn)
	}
}
