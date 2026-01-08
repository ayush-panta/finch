//go:build !darwin

// Package bridgecredhelper provides credential server functionality for Finch.
package bridgecredhelper

type DockerCredential struct {
	ServerURL string `json:"ServerURL"`
	Username  string `json:"Username"`
	Secret    string `json:"Secret"`
}

// CallCredentialHelper is a no-op on non-Darwin platforms
func CallCredentialHelper(action, serverURL, username, password string) (*DockerCredential, error) {
	return &DockerCredential{ServerURL: serverURL}, nil
}

// CallCredentialHelperWithEnv is a no-op on non-Darwin platforms
func CallCredentialHelperWithEnv(action, serverURL, username, password string, envVars map[string]string) (*DockerCredential, error) {
	return &DockerCredential{ServerURL: serverURL}, nil
}

// StartCredentialServer is a no-op on non-Darwin platforms
func StartCredentialServer(finchRootPath string) error {
	return nil
}

// StopCredentialServer is a no-op on non-Darwin platforms
func StopCredentialServer() {
	// No-op
}