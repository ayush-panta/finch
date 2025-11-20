package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
	"os/user"

	// dockertypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker-credential-helpers/credentials"
)

func RunCredScript() {
	// With inetdCompatibility, launchd passes a pre-connected socket as fd 0 (stdin)
	conn, err := net.FileConn(os.Stdin)
	if err != nil {
		log.Fatalf("Failed to get connection from stdin: %v", err)
	}
	defer conn.Close()

	current, _ := user.Current()
	log.Printf("Running as: %s (UID: %s)", current.Username, current.Uid)
	
	// Log keychain information
	keychainCmd := exec.Command("security", "list-keychains")
	keychainOutput, _ := keychainCmd.CombinedOutput()
	log.Printf("Available keychains: %s", strings.TrimSpace(string(keychainOutput)))
	
	// Log default keychain
	defaultCmd := exec.Command("security", "default-keychain")
	defaultOutput, _ := defaultCmd.CombinedOutput()
	log.Printf("Default keychain: %s", strings.TrimSpace(string(defaultOutput)))

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
	cmd := exec.Command("/Users/ayushkp/Documents/finch-creds/finch/_output/bin/cred-helpers/docker-credential-osxkeychain", command)
	cmd.Stdin = strings.NewReader(input)
	
	// Ensure keychain access by inheriting user environment
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	response := strings.TrimSpace(string(output))

	// Log raw output and error details
	log.Printf("Raw output: %q", string(output))
	log.Printf("Error: %v", err)
	log.Printf("Exit code: %v", cmd.ProcessState)

	if err != nil {
		log.Printf("Credential helper failed - stderr: %s", response)
		// For get commands, return empty credential helper format
		if command == "get" {
			emptyCreds := credentials.Credentials{ServerURL: input, Username: "", Secret: ""}
			credsJSON, _ := json.Marshal(emptyCreds)
			log.Printf("Returning empty credentials: %s", string(credsJSON))
			return string(credsJSON), nil
		}
		return response, nil
	}

	log.Printf("Credential helper SUCCESS - response: %s", response)

	// Return the raw response for successful get commands
	return response, nil
}

func main() {
	// Debug log to verify service starts (only to file, not stdout)
	f, _ := os.OpenFile("/tmp/cred-debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	fmt.Fprintf(f, "=== Credential Helper (inetd mode) ===\n")
	fmt.Fprintf(f, "Time: %s\n", time.Now().Format("3:04pm"))
	fmt.Fprintf(f, "Service started at %s\n", time.Now())
	fmt.Fprintf(f, "XPC_SERVICE_NAME: %s\n", os.Getenv("XPC_SERVICE_NAME"))
	fmt.Fprintf(f, "LAUNCH_DAEMON_SOCKET_NAME: %s\n", os.Getenv("LAUNCH_DAEMON_SOCKET_NAME"))
	
	// Test if stdin is a network connection (inetd style)
	if _, err := net.FileConn(os.Stdin); err == nil {
		fmt.Fprintf(f, "SUCCESS: stdin is a network connection\n")
		f.Close()
		
		// Also log to plist location
		plistLog, _ := os.OpenFile("/Users/ayushkp/Documents/finch-creds/finch/cred-helper.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		fmt.Fprintf(plistLog, "=== Service triggered at %s ===\n", time.Now().Format("3:04pm"))
		plistLog.Close()
		
		RunCredScript()
	} else {
		fmt.Fprintf(f, "ERROR: stdin is not a network connection: %v\n", err)
		fmt.Fprintf(f, "Exiting cleanly - this should only run with socket connections\n")
		f.Close()
		// Exit cleanly instead of crashing
		os.Exit(0)
	}
}
