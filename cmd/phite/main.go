package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	composercatalog "github.com/ohmiler/phite/composer"
	"github.com/ohmiler/phite/internal/managedcomposer"
	"github.com/ohmiler/phite/internal/managedruntime"
	runtimecatalog "github.com/ohmiler/phite/runtime"
)

var version = "dev"
var runtimeManifestPath string
var composerManifestPath string

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
	if len(arguments) >= 1 && arguments[0] == "composer" {
		return runComposer(arguments[1:])
	}
	fmt.Fprintln(os.Stderr, "Usage: phite <version|php|composer> [arguments...]")
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
	return runChild("phite php", installation.PHP, installation.Environment(os.Environ()), arguments)
}

func runComposer(arguments []string) int {
	runtime, err := runtimeManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "phite composer: %v\n", err)
		return 1
	}
	composer, err := composerManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "phite composer: %v\n", err)
		return 1
	}
	installation, err := runtime.Acquire(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "phite composer: %v\n", err)
		return 1
	}
	composerInstallation, err := composer.Acquire(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "phite composer: %v\n", err)
		return 1
	}
	childArguments := append([]string{composerInstallation.PHAR}, arguments...)
	return runChild("phite composer", installation.PHP, installation.Environment(os.Environ()), childArguments)
}

func runChild(label, executable string, environment, arguments []string) int {
	command := exec.Command(executable, arguments...)
	command.Env = environment
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	fmt.Fprintf(os.Stderr, "%s: execute Managed Runtime: %v\n", label, err)
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

func composerManager() (*managedcomposer.Manager, error) {
	manifestData := composercatalog.Manifest()
	if composerManifestPath != "" {
		var err error
		manifestData, err = os.ReadFile(composerManifestPath)
		if err != nil {
			return nil, fmt.Errorf("read Composer Manifest: %w", err)
		}
	}
	cacheRoot, err := managedcomposer.DefaultCacheRoot()
	if err != nil {
		return nil, err
	}
	return managedcomposer.New(manifestData, cacheRoot)
}
