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

	// Erase credentials using native helper
	_, err = callCredentialHelper("erase", registryURL.Host, "", "")
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to logout from %s: %v\n", registryURL.Host, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removed login credentials for %s\n", registryURL.Host)
	return nil
}