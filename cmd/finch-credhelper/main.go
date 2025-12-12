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

func main() {
	switch runtime.GOOS {
	case "darwin":
		darwinKeychainHandler()
	case "windows":
		windowsKeychainHandler()
	default:
        log.Printf("Unsupported platform: %s", runtime.GOOS)
        os.Exit(1)
	}
}
