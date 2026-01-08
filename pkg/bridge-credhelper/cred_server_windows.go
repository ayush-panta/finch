//go:build windows

// Package bridgecredhelper provides credential server functionality for Finch.
package bridgecredhelper

// StartCredentialServer is a no-op on Windows
func StartCredentialServer(finchRootPath string) error {
	return nil
}

// StopCredentialServer is a no-op on Windows
func StopCredentialServer() {
	// No-op
}