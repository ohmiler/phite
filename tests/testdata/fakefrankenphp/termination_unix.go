//go:build !windows

package main

import (
	"os/signal"
	"syscall"
)

func ignoreTermination() {
	signal.Ignore(syscall.SIGTERM)
}
