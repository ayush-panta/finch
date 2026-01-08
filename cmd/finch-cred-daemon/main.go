package main

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/runfinch/finch/pkg/bridge-credhelper"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: finch-cred-daemon <socket-path>")
	}
	
	socketPath := os.Args[1]
	
	// Clean up old socket
	os.Remove(socketPath)
	
	// Create listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Failed to create socket: %v", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)
	
	// Set permissions
	os.Chmod(socketPath, 0600)
	
	log.Printf("Credential daemon listening on %s", socketPath)
	
	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		listener.Close()
		os.Exit(0)
	}()
	
	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			break
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	
	// Read request
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	
	request := string(buf[:n])
	lines := strings.Split(strings.TrimSpace(request), "\n")
	if len(lines) < 2 || lines[0] != "get" {
		return
	}
	
	serverURL := lines[1]
	
	// Call the existing credential helper function directly
	creds, err := bridgecredhelper.CallCredentialHelper("get", serverURL, "", "")
	if err != nil {
		// Return empty credentials on error
		creds = &bridgecredhelper.DockerCredential{ServerURL: serverURL}
	}
	
	// Return JSON response
	data, _ := json.Marshal(creds)
	conn.Write(data)
}