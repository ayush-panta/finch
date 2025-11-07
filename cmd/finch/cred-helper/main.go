package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	dockertypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker-credential-helpers/credentials"
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
	fmt.Println("Credential Helper Daemon")
	fmt.Println("Time: ", time.Now().Format("3:04pm"))
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

			// Parse command and input
			if len(lines) < 2 {
				conn.Write([]byte("Error: invalid message format"))
				return
			}

			command := lines[0] // get, store, erase
			input := lines[1]   // JSON or server URL

			// Forward to real docker-credential-osxkeychain
			response, _ := forwardToCredHelper(command, input)
			
			// Handle "credentials not found" case - return empty for nerdctl compatibility
			if strings.Contains(response, "credentials not found") {
				response = ""
				log.Printf("Credentials not found - returning empty response for nerdctl compatibility")
			} else {
				log.Printf("Response to VM: %q", response)
			}
			log.Println("") // Add space between chunks

			// Send back response
			// dummy_response := "THIS IS A DUMMY RESPONSE"
			// log.Printf("Sending dummy response to VM: %q", dummy_response)
			
			conn.Write([]byte(response))
		}(conn)
	}
}

func forwardToCredHelper(command, input string) (string, error) {
	log.Printf("Forwarding command: %s, input: %s", command, input)

	// Execute docker-credential-osxkeychain with the command
	cmd := exec.Command("docker-credential-osxkeychain", command)
	cmd.Stdin = strings.NewReader(input)

	output, err := cmd.CombinedOutput()
	response := strings.TrimSpace(string(output))
	
	if err != nil {
		log.Printf("Credential helper stderr: %s", response)
		// For get commands, return empty AuthConfig instead of error string
		if command == "get" {
			emptyAuth := dockertypes.AuthConfig{ServerAddress: input}
			authJSON, _ := json.Marshal(emptyAuth)
			return string(authJSON), nil
		}
		return response, nil
	}

	log.Printf("Credential helper response: %s", response)

	// For get commands with successful responses, convert to AuthConfig
	if command == "get" && response != "" {
		var creds credentials.Credentials
		if err := json.Unmarshal([]byte(response), &creds); err == nil {
			// Convert to Docker AuthConfig
			authConfig := dockertypes.AuthConfig{
				Username:      creds.Username,
				Password:      creds.Secret,
				ServerAddress: creds.ServerURL,
			}
			
			authJSON, err := json.Marshal(authConfig)
			if err == nil {
				return string(authJSON), nil
			}
		}
	}
	
	return response, nil
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
