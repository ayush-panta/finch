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

const sockAddress = "/tmp/cred.sock"

func main() {

	// start the socket
	fmt.Println("Credential helper daemon started")
	socket, err := net.Listen("unix", sockAddress)
	if err != nil {
		log.Fatal(err)
	}

	// cleanup of socket as goroutine
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Remove(sockAddress)
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

			fmt.Printf("Command: '%s'\n", cmdName)
			fmt.Printf("Stdin data: '%s'\n", stdinData)

			binaryPath := "/Users/ayushkp/Documents/finch-creds/finch/_output/bin/cred-helpers/docker-credential-osxkeychain"
			cmd := exec.Command(binaryPath, cmdName)
			if stdinData != "" {
				cmd.Stdin = strings.NewReader(stdinData)
			}

			output, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}

			fmt.Printf("Output: %s\n", output)

			conn.Write(output)
		}(conn)
	}
}
