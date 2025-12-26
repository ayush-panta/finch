//go:build darwin || windows

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"

	"golang.org/x/net/context/ctxhttp"
	"golang.org/x/term"
	"github.com/spf13/cobra"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/core/remotes/docker/config"
	"github.com/containerd/errdefs"
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
	if credentials.Username == "" {
		fmt.Fprint(stdout, "Enter Username: ")
		username, err := readUsername()
		if err != nil {
			return err
		}
		credentials.Username = strings.TrimSpace(username)
	}

	if credentials.Password == "" {
		fmt.Fprint(stdout, "Enter Password: ")
		password, err := readPassword()
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout) // New line after password
		credentials.Password = strings.TrimSpace(password)
	}

	if credentials.Username == "" || credentials.Password == "" {
		return errors.New("username and password required")
	}

	// Validate credentials against registry using nerdctl's method
	err = loginClientSide(context.Background(), registryURL, credentials)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Store credentials using native helper
	err = credStore.Store(registryURL, credentials)
	if err != nil {
		return fmt.Errorf("error saving credentials: %w", err)
	}

	_, err = fmt.Fprintln(stdout, "Login Succeeded")
	return err
}

func readUsername() (string, error) {
	fd := os.Stdin
	if !term.IsTerminal(int(fd.Fd())) {
		return "", errors.New("stdin is not a terminal")
	}
	username, err := bufio.NewReader(fd).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(username), nil
}

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
		return "", err
	}
	return string(bytePassword), nil
}

// loginClientSide validates credentials using nerdctl's approach
func loginClientSide(ctx context.Context, registryURL *dockerconfigresolver.RegistryURL, credentials *dockerconfigresolver.Credentials) error {
	host := registryURL.Host
	
	authCreds := func(acArg string) (string, string, error) {
		if acArg == host {
			return credentials.Username, credentials.Password, nil
		}
		return "", "", fmt.Errorf("expected acArg to be %q, got %q", host, acArg)
	}

	dOpts := []dockerconfigresolver.Opt{
		dockerconfigresolver.WithAuthCreds(authCreds),
	}
	
	ho, err := dockerconfigresolver.NewHostOptions(ctx, host, dOpts...)
	if err != nil {
		return err
	}
	
	regHosts, err := config.ConfigureHosts(ctx, *ho)(host)
	if err != nil {
		return err
	}
	
	if len(regHosts) == 0 {
		return fmt.Errorf("got empty []docker.RegistryHost for %q", host)
	}
	
	for _, rh := range regHosts {
		err = tryLoginWithRegHost(ctx, rh)
		if err == nil {
			return nil
		}
	}
	return err
}

func tryLoginWithRegHost(ctx context.Context, rh docker.RegistryHost) error {
	if rh.Authorizer == nil {
		return errors.New("got nil Authorizer")
	}
	
	if rh.Path == "/v2" {
		rh.Path = "/v2/"
	}
	
	u := url.URL{
		Scheme: rh.Scheme,
		Host:   rh.Host,
		Path:   rh.Path,
	}
	
	var ress []*http.Response
	for i := 0; i < 10; i++ {
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			return err
		}
		
		for k, v := range rh.Header.Clone() {
			for _, vv := range v {
				req.Header.Add(k, vv)
			}
		}
		
		if err := rh.Authorizer.Authorize(ctx, req); err != nil {
			return fmt.Errorf("failed to authorize: %w", err)
		}
		
		res, err := ctxhttp.Do(ctx, rh.Client, req)
		if err != nil {
			return fmt.Errorf("failed to make request: %w", err)
		}
		
		ress = append(ress, res)
		
		if res.StatusCode == 401 {
			if err := rh.Authorizer.AddResponses(ctx, ress); err != nil && !errdefs.IsNotImplemented(err) {
				return fmt.Errorf("failed to add responses: %w", err)
			}
			continue
		}
		
		if res.StatusCode/100 != 2 {
			return fmt.Errorf("unexpected status code %d", res.StatusCode)
		}
		
		return nil
	}
	
	return errors.New("authentication failed after retries")
}