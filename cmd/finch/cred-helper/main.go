package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"os/user"

	dockertypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker-credential-helpers/credentials"
)

func RunCredScript() {

	// start the socket
	fmt.Println("Credential Helper Daemon")
	fmt.Println("Time: ", time.Now().Format("3:04pm"))

	// launchd passes socket as file descriptor 3
	file := os.NewFile(3, "socket")
	listener, err := net.FileListener(file)
	if err != nil {
		log.Fatalf("Failed to create listener from file descriptor: %v", err)
	}

	current, _ := user.Current()
	log.Printf("Running as: %s (UID: %s)", current.Username, current.Uid)

	// simply get one connection
	conn, err := listener.Accept()
	if err != nil {
		log.Fatalf("Failed to accept connection: %v", err)
	}
	defer conn.Close()

	// buffer for reading socket
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Printf("Read error: %v", err)
		return
	}

	// need to read in the data
	message := strings.TrimSpace(string(buffer[:n]))
	lines := strings.Split(message, "\n")
	if len(lines) == 0 {
		conn.Write([]byte("Error: empty message"))
		return
	}

	// Parse command and input
	if len(lines) != 2 {
		conn.Write([]byte("Error: invalid message format"))
		return
	}
	command := lines[0] // get, store, erase
	input := lines[1]   // JSON or server URL

	response, _ := forwardToCredHelper(command, input)

	// Handle "credentials not found" case - return empty for nerdctl compatibility
	if strings.Contains(response, "credentials not found") {
		response = ""
		log.Printf("Credentials not found - returning empty dockertypes.AuthConfig")
	} else {
		log.Printf("Response to VM: %q", response)
	}
	log.Println("") // Add space between chunks

	conn.Write([]byte(response))
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

func main() {
	RunCredScript()
}
