//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker-credential-helpers/credentials"
)

func windowsKeychainHandler() {
	// Test if stdin is a network connection (inetd style)
	if _, err := net.FileConn(os.Stdin); err == nil {
		if err := handleCredstoreRequest(); err != nil {
			os.Exit(1)
		}
	} else {
		os.Exit(0)
	}
}