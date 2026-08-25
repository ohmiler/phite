package devsession

import (
	"fmt"
	"os/exec"
	"runtime"
)

func OpenBrowser(endpoint string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", endpoint)
	case "darwin":
		command = exec.Command("open", endpoint)
	case "linux":
		command = exec.Command("xdg-open", endpoint)
	default:
		return fmt.Errorf("unsupported browser platform %s", runtime.GOOS)
	}
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}
