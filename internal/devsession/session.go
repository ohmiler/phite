package devsession

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"time"
)

const defaultStartupTimeout = 15 * time.Second

type Project struct {
	Directory    string
	DocumentRoot string
	Entrypoint   string
}

type Options struct {
	Project        Project
	FrankenPHP     string
	Environment    []string
	Input          io.Reader
	Output         io.Writer
	ErrorOutput    io.Writer
	OpenBrowser    func(string) error
	StartupTimeout time.Duration
}

type processState struct {
	done chan struct{}
	err  error
}

func DiscoverProject(directory string) (Project, error) {
	return discoverProject(directory)
}

func Run(ctx context.Context, options Options) error {
	if err := validateOptions(options); err != nil {
		return err
	}
	timeout := options.StartupTimeout
	if timeout <= 0 {
		timeout = defaultStartupTimeout
	}

	address, endpoint, err := availableEndpoint()
	if err != nil {
		return fmt.Errorf("select loopback Local Endpoint: %w", err)
	}
	command := exec.Command(
		options.FrankenPHP,
		"php-server",
		"--listen="+address,
		"--root="+options.Project.DocumentRoot,
	)
	command.Dir = options.Project.Directory
	command.Env = options.Environment
	command.Stdout = options.ErrorOutput
	command.Stderr = options.ErrorOutput
	configureProcess(command)
	if err := command.Start(); err != nil {
		return fmt.Errorf("start FrankenPHP Required Capability: %w", err)
	}

	state := &processState{done: make(chan struct{})}
	go func() {
		state.err = command.Wait()
		close(state.done)
	}()
	if err := waitUntilReady(ctx, address, timeout, state); err != nil {
		return errors.Join(err, stopAndWait(command, state))
	}
	fmt.Fprintf(options.Output, "Local Endpoint: %s\n", endpoint)
	fmt.Fprintln(options.Output, "Press o then Enter to open it; press Ctrl+C to stop.")

	openRequests := make(chan struct{}, 1)
	go readInteractiveCommands(options.Input, openRequests)
	for {
		select {
		case <-ctx.Done():
			return stopAndWait(command, state)
		case <-state.done:
			if state.err == nil {
				return errors.New("FrankenPHP Required Capability stopped unexpectedly")
			}
			return fmt.Errorf("FrankenPHP Required Capability stopped unexpectedly: %w", state.err)
		case <-openRequests:
			if err := options.OpenBrowser(endpoint); err != nil {
				fmt.Fprintf(options.ErrorOutput, "phite dev: open Local Endpoint: %v\n", err)
			}
		}
	}
}

func validateOptions(options Options) error {
	if options.Project.Directory == "" || options.Project.DocumentRoot == "" || options.Project.Entrypoint == "" {
		return errors.New("start Development Session: PHP Project is incomplete")
	}
	if strings.TrimSpace(options.FrankenPHP) == "" {
		return errors.New("start FrankenPHP Required Capability: executable path is empty")
	}
	if options.Input == nil || options.Output == nil || options.ErrorOutput == nil {
		return errors.New("start Development Session: terminal streams are required")
	}
	if options.OpenBrowser == nil {
		return errors.New("start Development Session: browser opener is required")
	}
	return nil
}

func availableEndpoint() (string, string, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", "", err
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		return "", "", err
	}
	return address, "http://" + address, nil
}

func waitUntilReady(ctx context.Context, address string, timeout time.Duration, state *processState) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	dialer := &net.Dialer{Timeout: 250 * time.Millisecond}
	for {
		connection, err := dialer.DialContext(ctx, "tcp4", address)
		if err == nil {
			connection.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("start FrankenPHP Required Capability: %w", ctx.Err())
		case <-state.done:
			if state.err == nil {
				return errors.New("start FrankenPHP Required Capability: process stopped before the Local Endpoint became ready")
			}
			return fmt.Errorf("start FrankenPHP Required Capability: process stopped before the Local Endpoint became ready: %w", state.err)
		case <-deadline.C:
			return fmt.Errorf("start FrankenPHP Required Capability: Local Endpoint http://%s was not ready within %s", address, timeout)
		case <-ticker.C:
		}
	}
}

func stopAndWait(command *exec.Cmd, state *processState) error {
	select {
	case <-state.done:
		return nil
	default:
	}
	stopProcess(command)
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-state.done:
		return nil
	case <-timer.C:
	}
	forceStopProcess(command)
	timer.Reset(3 * time.Second)
	select {
	case <-state.done:
		return nil
	case <-timer.C:
		return errors.New("stop FrankenPHP Required Capability: process did not exit after forced termination")
	}
}

func readInteractiveCommands(input io.Reader, openRequests chan<- struct{}) {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		if strings.EqualFold(strings.TrimSpace(scanner.Text()), "o") {
			select {
			case openRequests <- struct{}{}:
			default:
			}
		}
	}
}
