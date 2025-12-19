//go:build darwin || windows

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Extracted from nerdctl's prompt.go
var (
	ErrUsernameIsRequired = errors.New("username is required")
	ErrPasswordIsRequired = errors.New("password is required")
	ErrReadingUsername    = errors.New("unable to read username")
	ErrReadingPassword    = errors.New("unable to read password")
	ErrNotATerminal       = errors.New("stdin is not a terminal (Hint: use `finch login --username=USERNAME --password-stdin`)")
)

func newLoginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "login [flags] [SERVER]",
		Args:          cobra.MaximumNArgs(1),
		Short:         "Log in to a container registry",
		RunE:          loginAction,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().StringP("username", "u", "", "Username")
	cmd.Flags().StringP("password", "p", "", "Password")
	cmd.Flags().Bool("password-stdin", false, "Take the password from stdin")
	return cmd
}

func loginAction(cmd *cobra.Command, args []string) error {
	// Parse flags (extracted from nerdctl's loginOptions)
	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")
	passwordStdin, _ := cmd.Flags().GetBool("password-stdin")

	// Validation (extracted from nerdctl)
	if strings.Contains(username, ":") {
		return errors.New("username cannot contain colons")
	}

	if password != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "WARNING! Using --password via the CLI is insecure. Use --password-stdin.")
		if passwordStdin {
			return errors.New("--password and --password-stdin are mutually exclusive")
		}
	}

	if passwordStdin {
		if username == "" {
			return errors.New("must provide --username with --password-stdin")
		}
		contents, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return err
		}
		password = strings.TrimSuffix(string(contents), "\n")
		password = strings.TrimSuffix(password, "\r")
	}

	// Get server address
	serverAddress := ""
	if len(args) == 1 {
		serverAddress = args[0]
	}

	// Parse registry URL (simplified from nerdctl's dockerconfigresolver.Parse)
	registryHost, err := parseRegistryURL(serverAddress)
	if err != nil {
		return err
	}

	// Prompt for missing credentials (extracted from nerdctl's promptUserForAuthentication)
	if username == "" {
		fmt.Fprint(cmd.OutOrStdout(), "Enter Username: ")
		username, err = readUsername()
		if err != nil {
			return err
		}
		username = strings.TrimSpace(username)
		if username == "" {
			return ErrUsernameIsRequired
		}
	}

	if password == "" {
		fmt.Fprint(cmd.OutOrStdout(), "Enter Password: ")
		password, err = readPassword()
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout())
		password = strings.TrimSpace(password)
		if password == "" {
			return ErrPasswordIsRequired
		}
	}

	// Validate credentials with registry (simplified from nerdctl's loginClientSide)
	err = validateCredentials(registryHost, username, password)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Store credentials using native helper (REPLACE nerdctl's credStore.Store)
	err = callNativeCredHelper("store", registryHost, username, password)
	if err != nil {
		return fmt.Errorf("error saving credentials: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Login Succeeded")
	return nil
}

// Extracted from nerdctl's prompt.go
func readUsername() (string, error) {
	fd := os.Stdin
	if !term.IsTerminal(int(fd.Fd())) {
		return "", ErrNotATerminal
	}

	username, err := bufio.NewReader(fd).ReadString('\n')
	if err != nil {
		return "", errors.Join(ErrReadingUsername, err)
	}

	return strings.TrimSpace(username), nil
}

// Extracted from nerdctl's prompt_unix.go
func readPassword() (string, error) {
	fd := syscall.Stdin
	if !term.IsTerminal(fd) {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return "", err
		}
		defer tty.Close()
		fd = int(tty.Fd())
	}

	bytePassword, err := term.ReadPassword(fd)
	if err != nil {
		return "", errors.Join(ErrReadingPassword, err)
	}

	return string(bytePassword), nil
}



// Simplified version of nerdctl's loginClientSide validation
func validateCredentials(registryHost, username, password string) error {
	// Build registry API URL
	registryURL := "https://" + registryHost
	if registryHost == "index.docker.io" {
		registryURL = "https://registry-1.docker.io"
	}
	registryURL += "/v2/"

	// Create HTTP request
	req, err := http.NewRequest("GET", registryURL, nil)
	if err != nil {
		return err
	}

	// Add basic auth
	req.SetBasicAuth(username, password)

	// Make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to registry: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode == 401 {
		return errors.New("invalid credentials")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("registry error: %d", resp.StatusCode)
	}

	return nil
}

