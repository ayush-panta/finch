//go:build darwin || windows

package main

import (
	"fmt"

	"github.com/spf13/cobra"
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

	// Parse registry URL (same logic as login)
	registryHost, err := parseRegistryURL(serverAddress)
	if err != nil {
		return err
	}

	// Erase credentials using native helper (REPLACE nerdctl's credStore.Erase)
	err = callNativeCredHelper("erase", registryHost, "", "")
	if err != nil {
		return fmt.Errorf("failed to erase credentials for: %s - %w", registryHost, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removed login credentials for %s\n", registryHost)
	return nil
}

