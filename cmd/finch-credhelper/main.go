package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker-credential-helpers/credentials"
)

const (
	maxBufferSize = 4096
	timeFormat    = "15:04:05"
)

func handleCredentialRequest() error {
	conn, err := net.FileConn(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to get connection from stdin: %w", err)
	}
	defer conn.Close()

	current, err := user.Current()
	if err != nil {
		log.Printf("Warning: could not get current user: %v", err)
	} else {
		log.Printf("Running as: %s (UID: %s)", current.Username, current.Uid)
	}

	logCredentialStoreInfo()

	buffer := make([]byte, maxBufferSize)
	n, err := conn.Read(buffer)
	if err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	message := strings.TrimSpace(string(buffer[:n]))
	command, input, err := parseRequest(message)
	if err != nil {
		conn.Write([]byte(fmt.Sprintf("Error: %v", err)))
		return err
	}

	response, err := forwardToCredHelper(command, input)
	if err != nil {
		log.Printf("Credential helper error: %v", err)
	}

	// Handle "credentials not found" case - return empty for nerdctl compatibility
	if strings.Contains(response, "credentials not found") {
		response = ""
		log.Printf("Credentials not found - returning empty response")
	} else {
		log.Printf("Response to VM: %q", response)
	}

	_, writeErr := conn.Write([]byte(response))
	return writeErr
}

func parseRequest(message string) (command, input string, err error) {
	lines := strings.Split(message, "\n")
	if len(lines) == 0 {
		return "", "", fmt.Errorf("empty message")
	}

	if len(lines) < 1 {
		return "", "", fmt.Errorf("no command provided")
	}

	command = lines[0]
	if command == "list" {
		return command, "", nil
	}

	if len(lines) != 2 {
		return "", "", fmt.Errorf("command %s requires input", command)
	}

	return command, lines[1], nil
}

func logCredentialStoreInfo() {
	switch runtime.GOOS {
	case "darwin":
		if output, err := exec.Command("security", "list-keychains").CombinedOutput(); err == nil {
			log.Printf("Available keychains: %s", strings.TrimSpace(string(output)))
		}
	case "windows":
		if output, err := exec.Command("cmdkey", "/list").CombinedOutput(); err == nil {
			log.Printf("Available credentials: %s", strings.TrimSpace(string(output)))
		}
	}
}

func forwardToCredHelper(command, input string) (string, error) {
	log.Printf("Forwarding command: %s, input: %s", command, input)

	credHelperPath, err := getCredentialHelperPath()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(credHelperPath, command)
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	response := strings.TrimSpace(string(output))

	log.Printf("Raw output: %q", string(output))
	log.Printf("Error: %v", err)
	if cmd.ProcessState != nil {
		log.Printf("Exit code: %d", cmd.ProcessState.ExitCode())
	}

	if err != nil {
		log.Printf("Credential helper failed: %s", response)
		if command == "get" {
			emptyCreds := credentials.Credentials{ServerURL: input, Username: "", Secret: ""}
			credsJSON, _ := json.Marshal(emptyCreds)
			log.Printf("Returning empty credentials: %s", string(credsJSON))
			return string(credsJSON), nil
		}
		return response, err
	}

	log.Printf("Credential helper SUCCESS - response: %s", response)
	return response, nil
}

func getCredentialHelperPath() (string, error) {
	var helperName string
	switch runtime.GOOS {
	case "darwin":
		helperName = "docker-credential-osxkeychain"
	case "windows":
		helperName = "docker-credential-wincred.exe"
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// Runtime path detection: find helper relative to bridge executable
	// This works for both dev (_output/finch-credhelper/) and prod (/Applications/Finch/finch-credhelper/)
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Helper should be in same directory as bridge
	baseDir := filepath.Dir(execPath)
	helperPath := filepath.Join(baseDir, helperName)

	// Verify helper exists
	if _, err := os.Stat(helperPath); err == nil {
		log.Printf("Found credential helper at: %s", helperPath)
		return helperPath, nil
	}

	// Fallback: try common system locations for compatibility
	fallbackPaths := []string{
		"/usr/local/bin/" + helperName,
		"/Applications/Finch/bin/" + helperName,
	}

	for _, path := range fallbackPaths {
		if _, err := os.Stat(path); err == nil {
			log.Printf("Found credential helper at fallback location: %s", path)
			return path, nil
		}
	}

	return "", fmt.Errorf("%s not found in same directory (%s) or fallback locations", helperName, baseDir)
}

func getLogPath() string {
	if runtime.GOOS == "windows" {
		return "C:\\temp\\cred-debug.log"
	}
	return "/tmp/cred-debug.log"
}

func initializeLogging() {
	logPath := getLogPath()
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: could not open log file %s: %v", logPath, err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "=== Credential Helper (inetd mode) ===\n")
	fmt.Fprintf(f, "Time: %s\n", time.Now().Format(timeFormat))
	fmt.Fprintf(f, "Service started at %s\n", time.Now())
	fmt.Fprintf(f, "XPC_SERVICE_NAME: %s\n", os.Getenv("XPC_SERVICE_NAME"))
	fmt.Fprintf(f, "LAUNCH_DAEMON_SOCKET_NAME: %s\n", os.Getenv("LAUNCH_DAEMON_SOCKET_NAME"))
}

func main() {
	initializeLogging()

	// Test if stdin is a network connection (inetd style)
	if _, err := net.FileConn(os.Stdin); err == nil {
		log.Printf("SUCCESS: stdin is a network connection")
		if err := handleCredentialRequest(); err != nil {
			log.Printf("Error handling credential request: %v", err)
			os.Exit(1)
		}
	} else {
		log.Printf("ERROR: stdin is not a network connection: %v", err)
		log.Printf("Exiting cleanly - this should only run with socket connections")
		os.Exit(0)
	}
}
