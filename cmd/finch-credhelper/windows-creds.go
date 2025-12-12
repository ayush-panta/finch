//go:build windows

package main

import (
	"log"
	"os"
)

func windowsKeychainHandler() {
	log.Printf("Windows credential handler not implemented yet")
	os.Exit(1)
}

func main() {
	windowsKeychainHandler()
}