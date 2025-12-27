//go:build darwin || windows

package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/runfinch/finch/pkg/command"
)

func newLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "logout [flags] [SERVER]",
		Args:              cobra.MaximumNArgs(1),
		Short:             "Log out from a container registry",
		RunE:              logoutAction,
		SilenceUsage:      true,
		SilenceErrors:     true,
	}
}

func logoutAction(cmd *cobra.Command, args []string) error {
	// Get server address
	serverAddress := ""
	if len(args) > 0 {
		serverAddress = args[0]
	}

	// Logout using nerdctl logout directly
	err := command.NerdctlLogout(serverAddress, cmd.OutOrStdout())
	if err != nil {
		return fmt.Errorf("logout failed: %w", err)
	}

	return nil
}

