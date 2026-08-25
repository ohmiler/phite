//go:build !windows

package devsession

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestDiscoverProjectRejectsNamedPipeConfigurationWithoutBlocking(t *testing.T) {
	projectDirectory := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(projectDirectory, "phite.yaml"), 0o600); err != nil {
		t.Fatalf("create phite.yaml named pipe: %v", err)
	}

	_, err := DiscoverProject(projectDirectory)
	if err == nil || !strings.Contains(err.Error(), "phite.yaml is not a regular file") {
		t.Fatalf("expected regular-file error, got %v", err)
	}
}
