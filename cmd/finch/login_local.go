//go:build darwin || windows

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/containerd/nerdctl/v2/pkg/imgutil/dockerconfigresolver"
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
	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")
	passwordStdin, _ := cmd.Flags().GetBool("password-stdin")

	// Validation from nerdctl
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

	serverAddress := ""
	if len(args) == 1 {
		serverAddress = args[0]
	}

	// Use custom login with native credential store
	return loginWithNativeCredStore(serverAddress, username, password, cmd.OutOrStdout())
}

func loginWithNativeCredStore(serverAddress, username, password string, stdout io.Writer) error {
	// Parse registry URL using nerdctl's logic
	registryURL, err := dockerconfigresolver.Parse(serverAddress)
	if err != nil {
		return err
	}

	// Use custom credential store
	credStore := newCustomCredStore()

	// Create credentials
	credentials := &dockerconfigresolver.Credentials{
		Username: username,
		Password: password,
	}

	// Prompt for missing credentials if needed
	if credentials.Username == "" || credentials.Password == "" {
		// TODO: Add prompting logic similar to nerdctl
		return errors.New("username and password required")
	}

	// Store credentials using native helper
	err = credStore.Store(registryURL, credentials)
	if err != nil {
		return fmt.Errorf("error saving credentials: %w", err)
	}

	_, err = fmt.Fprintln(stdout, "Login Succeeded")
	return err