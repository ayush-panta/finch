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
	fmt.Println("Credential helper daemon started")

	socket, err := net.Listen("unix", sockAddress)
	if err != nil {
		log.Fatal(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Remove(sockAddress)
		os.Exit(1)
	}()

	for {
		conn, err := socket.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			buf := make([]byte, 4096)
			n, err := conn.Read(buf)
			if err != nil {
				log.Printf("Read error: %v", err)
				return
			}

			cmdStr := strings.TrimSpace(string(buf[:n]))
			fmt.Printf("Executing: %s\n", cmdStr)

			if cmdStr == "" {
				conn.Write([]byte("Error: empty command"))
				return
			}

			cmd := exec.Command("sh", "-c", cmdStr)
			output, err := cmd.CombinedOutput()
			if err != nil {
				result := fmt.Sprintf("Error: %v\nOutput: %s", err, string(output))
				conn.Write([]byte(result))
				return
			}

			conn.Write(output)
		}(conn)
	}
}
