//go:build darwin || windows

package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/runfinch/finch/pkg/command"
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

	// Store credentials using nerdctl login directly
	err = command.NerdctlLogin(serverAddress, username, password, cmd.OutOrStdout())
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	return nil
}

