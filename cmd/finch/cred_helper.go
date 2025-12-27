//go:build darwin || windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// From nerdctl's dockerconfigresolver
type scheme string

const (
	StandardHTTPSPort                = "443"
	schemeHTTPS               scheme = "https"
	schemeHTTP                scheme = "http"
	schemeNerdctlExperimental scheme = "nerdctl-experimental"
	dockerIndexServer                = "https://index.docker.io/v1/"
	namespaceQueryParameter          = "ns"
)

var (
	ErrUnparsableURL     = errors.New("unparsable registry URL")
	ErrUnsupportedScheme = errors.New("unsupported scheme in registry URL")
)

// RegistryURL from nerdctl's dockerconfigresolver
type RegistryURL struct {
	url.URL
	Namespace *RegistryURL
}

// Parse from nerdctl's dockerconfigresolver
func Parse(address string) (*RegistryURL, error) {
	var err error
	if address == "" || address == "docker.io" {
		address = dockerIndexServer
	}
	if !strings.Contains(address, "://") {
		address = fmt.Sprintf("%s://%s", schemeHTTPS, address)
	}
	u, err := url.Parse(address)
	if err != nil {
		return nil, errors.Join(ErrUnparsableURL, err)
	}
	sch := scheme(u.Scheme)
	if sch == schemeHTTP {
		u.Scheme = string(schemeHTTPS)
	} else if sch != schemeHTTPS && sch != schemeNerdctlExperimental {
		return nil, ErrUnsupportedScheme
	}
	if u.Port() == "" {
		u.Host = u.Hostname() + ":" + StandardHTTPSPort
	}
	reg := &RegistryURL{URL: *u}
	queryParams := u.Query()
	nsQuery := queryParams.Get(namespaceQueryParameter)
	if nsQuery != "" {
		reg.Namespace, err = Parse(nsQuery)
		if err != nil {
			return nil, err
		}
	}
	return reg, nil
}

// CanonicalIdentifier from nerdctl's dockerconfigresolver
func (rn *RegistryURL) CanonicalIdentifier() string {
	if rn.Scheme == string(schemeHTTPS) && rn.Hostname() == "index.docker.io" && rn.Path == "/v1/" && rn.Port() == StandardHTTPSPort ||
		rn.URL.String() == dockerIndexServer {
		return dockerIndexServer
	}
	identifier := rn.Host
	if rn.Namespace != nil {
		identifier = fmt.Sprintf("%s://%s/host/%s%s", schemeNerdctlExperimental, rn.Namespace.CanonicalIdentifier(), identifier, rn.Path)
	}
	return identifier
}

// Docker credential helper protocol
type dockerCredential struct {
	ServerURL string `json:"ServerURL"`
	Username  string `json:"Username"`
	Secret    string `json:"Secret"`
}

// Native credential helper call
func callNativeCredHelper(action, serverURL, username, password string) error {
	// Get finch directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Determine credential helper binary name based on OS
	var helperName string
	switch runtime.GOOS {
	case "darwin":
		helperName = "docker-credential-osxkeychain"
	case "windows":
		helperName = "docker-credential-wincred.exe"
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	// Build path to credential helper
	helperPath := filepath.Join(homeDir, ".finch", "cred-helpers", helperName)

	// Check if helper exists
	if _, err := os.Stat(helperPath); os.IsNotExist(err) {
		return fmt.Errorf("credential helper not found: %s", helperPath)
	}

	// Create command
	cmd := exec.Command(helperPath, action)

	// For store action, send credential data via stdin
	if action == "store" {
		cred := dockerCredential{
			ServerURL: serverURL,
			Username:  username,
			Secret:    password,
		}

		credJSON, err := json.Marshal(cred)
		if err != nil {
			return fmt.Errorf("failed to marshal credentials: %w", err)
		}

		cmd.Stdin = strings.NewReader(string(credJSON))
	} else if action == "get" || action == "erase" {
		// For get/erase actions, send server URL via stdin
		cmd.Stdin = strings.NewReader(serverURL)
	}

	// Run command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("credential helper failed: %w - %s", err, string(output))
	}

	return nil
}

// callNativeCredHelperWithOutput calls the native credential helper and returns the parsed credentials
func callNativeCredHelperWithOutput(action, serverURL, username, password string) (*dockerCredential, error) {
	// Get finch directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Determine credential helper binary name based on OS
	var helperName string
	switch runtime.GOOS {
	case "darwin":
		helperName = "docker-credential-osxkeychain"
	case "windows":
		helperName = "docker-credential-wincred.exe"
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	// Build path to credential helper
	helperPath := filepath.Join(homeDir, ".finch", "cred-helpers", helperName)

	// Check if helper exists
	if _, err := os.Stat(helperPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("credential helper not found: %s", helperPath)
	}

	// Create command
	cmd := exec.Command(helperPath, action)

	// For get action, send server URL via stdin
	if action == "get" {
		cmd.Stdin = strings.NewReader(serverURL)
	} else if action == "store" {
		// For store action, send credential data via stdin
		cred := dockerCredential{
			ServerURL: serverURL,
			Username:  username,
			Secret:    password,
		}

		credJSON, err := json.Marshal(cred)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal credentials: %w", err)
		}

		cmd.Stdin = strings.NewReader(string(credJSON))
	} else if action == "erase" {
		cmd.Stdin = strings.NewReader(serverURL)
	}

	// Run command and capture output
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("credential helper failed: %w", err)
	}

	// For get action, parse the JSON response
	if action == "get" {
		var creds dockerCredential
		if err := json.Unmarshal(output, &creds); err != nil {
			return nil, fmt.Errorf("failed to parse credential response: %w", err)
		}
		return &creds, nil
	}

	// For other actions, return empty credentials
	return &dockerCredential{}, nil
}

// parseRegistryURL uses nerdctl's actual Parse function
func parseRegistryURL(serverAddress string) (string, error) {
	reg, err := Parse(serverAddress)
	if err != nil {
		return "", err
	}
	return reg.CanonicalIdentifier(), nil
}
