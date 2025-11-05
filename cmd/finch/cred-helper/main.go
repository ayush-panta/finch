package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const hostAddr = "127.0.0.1:8080" // Listen on localhost only

// Lima VM IP ranges to allow
var allowedNetworks = []string{
	"192.168.5.0/24",   // Lima default network
	"192.168.105.0/24", // Lima alternative network
	"127.0.0.1/32",     // Localhost
}

func StartServer() {
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
		// accept the incoming connection
		conn, err := socket.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}

		// Check if connection is from allowed IP
		remoteAddr := conn.RemoteAddr().(*net.TCPAddr)
		if !isAllowedIP(remoteAddr.IP) {
			log.Printf("Rejected connection from %s", remoteAddr.IP)
			conn.Close()
			continue
		}

		// goroutine for processing
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

			// dummy output for lines in
			for i, line := range lines {
				fmt.Println(i, line)
			}

			// no return right niw
			// conn.Write(output)
		}(conn)
	}
}

func isAllowedIP(ip net.IP) bool {
	for _, network := range allowedNetworks {
		_, cidr, err := net.ParseCIDR(network)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func main() {
	StartServer()
}
