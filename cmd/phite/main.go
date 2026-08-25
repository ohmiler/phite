package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/ohmiler/phite/internal/managedruntime"
	runtimecatalog "github.com/ohmiler/phite/runtime"
)

var version = "dev"
var runtimeManifestPath string

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(arguments []string) int {
	if len(arguments) == 1 && arguments[0] == "version" {
		return runVersion()
	}
	if len(arguments) >= 1 && arguments[0] == "php" {
		return runPHP(arguments[1:])
	}

	fmt.Fprintln(os.Stderr, "Usage: phite <version|php> [arguments...]")
	return 2
}

func runVersion() int {
	fmt.Printf("Phite CLI %s\n", version)
	manager, err := runtimeManager()
	if errors.Is(err, managedruntime.ErrUnsupportedPlatform) {
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "phite version: %v\n", err)
		return 1
	}
	identity, installed, err := manager.Installed()
	if err != nil {
		fmt.Fprintf(os.Stderr, "phite version: %v\n", err)
		return 1
	}
	if installed {
		fmt.Printf("Managed Runtime %s\n", identity.ID)
	}
	return 0
}

func runPHP(arguments []string) int {
	manager, err := runtimeManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "phite php: %v\n", err)
		return 1
	}
	installation, err := manager.Acquire(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "phite php: %v\n", err)
		return 1
	}

	command := exec.Command(installation.PHP, arguments...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err = command.Run()
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	fmt.Fprintf(os.Stderr, "phite php: execute Managed Runtime: %v\n", err)
	return 1
}

func runtimeManager() (*managedruntime.Manager, error) {
	manifestData := runtimecatalog.Manifest()
	if runtimeManifestPath != "" {
		var err error
		manifestData, err = os.ReadFile(runtimeManifestPath)
		if err != nil {
			return nil, fmt.Errorf("read Runtime Manifest: %w", err)
		}
	}
	cacheRoot, err := managedruntime.DefaultCacheRoot()
	if err != nil {
		return nil, err
	}
	return managedruntime.New(manifestData, cacheRoot)
}
