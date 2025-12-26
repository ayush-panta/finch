//go:build darwin || windows

package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/containerd/nerdctl/v2/pkg/imgutil/dockerconfigresolver"
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
	serverAddress := ""
	if len(args) > 0 {
		serverAddress = args[0]
	}

	// Parse registry URL
	registryURL, err := dockerconfigresolver.Parse(serverAddress)
	if err != nil {
		return err
	}

	// Use custom credential store
	credStore := newCustomCredStore()

	// Erase credentials using native helper
	errs, err := credStore.Erase(registryURL)
	if err != nil {
		return fmt.Errorf("logout failed: %w", err)
	}

	// Handle any per-server errors
	for server, serverErr := range errs {
		if serverErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to logout from %s: %v\n", server, serverErr)
		}
	}

	if serverAddress != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Removed login credentials for %s\n", serverAddress)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Removed login credentials")
	}

	return nil
}