// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Basic thing

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

const sockAddress = "/tmp/cred.sock"

func main() {

	fmt.Println("This is the credential helper daemon.")

	// Creating the socket and ensuring success
	socket, err := net.Listen("unix", sockAddress)
	if err != nil {
		log.Fatal(err) // Fatal is fine -> unsuccessful
	}

	// Cleanup sockfile with go routine (?)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Remove(sockAddress)
		os.Exit(1)
	}()

	for {
		// Accept and validate a connection
		conn, err := socket.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err) // instead of Fatal
		}

		go func(conn net.Conn) {
			// close at the end
			defer conn.Close()

			// create buffer for unix socket
			buf := make([]byte, 4096)

			// read and display data in string fmt
			n, err := conn.Read(buf)
			if err != nil {
				log.Printf("Read error: %v", err) // instead of Fatal
				return
			}
			fmt.Println("Received: ", string(buf[:n]))

			// write data back
			_, err = conn.Write(buf[:n])
			if err != nil {
				log.Printf("Write error: %v", err) // instead of Fatal
				return
			}
		}(conn)
	}
}
