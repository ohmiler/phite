//go:build !windows

package devsession

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func stopProcess(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	err := syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
	if err != nil && !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
		_ = command.Process.Kill()
	}
}

func forceStopProcess(command *exec.Cmd) {
	if command.Process != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
}
