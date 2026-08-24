package cli_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVersionReportsPhiteCLIWithoutManagedRuntime(t *testing.T) {
	binary := buildPhite(t)
	projectDir := t.TempDir()

	command := exec.Command(binary, "version")
	command.Dir = projectDir
	command.Env = []string{"PHITE_RUNTIME_CACHE=" + filepath.Join(t.TempDir(), "runtime-cache")}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		t.Fatalf("run phite version: %v\nstderr:\n%s", err, stderr.String())
	}

	if got, want := stdout.String(), "Phite CLI dev\n"; got != want {
		t.Fatalf("stdout mismatch\nwant: %q\n got: %q", want, got)
	}

	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func buildPhite(t *testing.T) string {
	t.Helper()

	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}

	name := "phite"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	binary := filepath.Join(t.TempDir(), name)
	command := exec.Command("go", "build", "-o", binary, "./cmd/phite")
	command.Dir = projectRoot

	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build phite: %v\n%s", err, output)
	}

	return binary
}
