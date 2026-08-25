package devsession

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDiscoverProjectUsesConventionalFrontController(t *testing.T) {
	for _, test := range []struct {
		name       string
		entrypoint string
		wantRoot   string
	}{
		{name: "public directory", entrypoint: "public/index.php", wantRoot: "public"},
		{name: "project root", entrypoint: "index.php", wantRoot: "."},
	} {
		t.Run(test.name, func(t *testing.T) {
			projectDirectory := t.TempDir()
			entrypoint := filepath.Join(projectDirectory, filepath.FromSlash(test.entrypoint))
			if err := os.MkdirAll(filepath.Dir(entrypoint), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(entrypoint, []byte("<?php echo 'fixture';\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			project, err := DiscoverProject(projectDirectory)
			if err != nil {
				t.Fatalf("discover project: %v", err)
			}
			if got, want := project.DocumentRoot, filepath.Clean(filepath.Join(projectDirectory, test.wantRoot)); got != want {
				t.Fatalf("Document Root mismatch: want %q, got %q", want, got)
			}
			if got, want := project.Entrypoint, entrypoint; got != want {
				t.Fatalf("Entrypoint mismatch: want %q, got %q", want, got)
			}
		})
	}
}

func TestDiscoverProjectExplainsMissingEntrypoint(t *testing.T) {
	_, err := DiscoverProject(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "public/index.php or index.php") {
		t.Fatalf("expected actionable Entrypoint error, got %v", err)
	}
}

func TestRunProvidesLoopbackEndpointRoutesRequestsAndCleansUp(t *testing.T) {
	projectDirectory := t.TempDir()
	publicDirectory := filepath.Join(projectDirectory, "public")
	if err := os.MkdirAll(publicDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(publicDirectory, "index.php"), "<?php echo 'front controller';\n")
	writeTestFile(t, filepath.Join(publicDirectory, "app.css"), "body { color: green; }\n")
	before := snapshotFiles(t, projectDirectory)

	project, err := DiscoverProject(projectDirectory)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	inputReader, inputWriter := io.Pipe()
	defer inputWriter.Close()
	var output lockedBuffer
	var errorsOutput lockedBuffer
	opened := make(chan string, 1)
	done := make(chan error, 1)
	requestMarker := filepath.Join(t.TempDir(), "request.marker")
	go func() {
		done <- Run(ctx, Options{
			Project:        project,
			FrankenPHP:     buildFakeFrankenPHP(t),
			Environment:    append(os.Environ(), "FAKE_FRANKENPHP_REQUEST_MARKER="+requestMarker),
			Input:          inputReader,
			Output:         &output,
			ErrorOutput:    &errorsOutput,
			OpenBrowser:    func(endpoint string) error { opened <- endpoint; return nil },
			StartupTimeout: 5 * time.Second,
		})
	}()

	endpoint := waitForEndpoint(t, &output)
	if !strings.HasPrefix(endpoint, "http://127.0.0.1:") {
		t.Fatalf("Local Endpoint is not loopback-only: %q", endpoint)
	}
	if _, err := os.Stat(requestMarker); !os.IsNotExist(err) {
		t.Fatalf("readiness probe invoked the Entrypoint before printing the Local Endpoint: %v", err)
	}
	select {
	case unexpected := <-opened:
		t.Fatalf("browser opened automatically at %s", unexpected)
	default:
	}

	assertResponse(t, endpoint+"/app.css", "body { color: green; }\n")
	assertResponse(t, endpoint+"/missing/route", "front controller:/missing/route")

	if _, err := io.WriteString(inputWriter, "o\n"); err != nil {
		t.Fatalf("send interactive command: %v", err)
	}
	select {
	case got := <-opened:
		if got != endpoint {
			t.Fatalf("opened endpoint mismatch: want %q, got %q", endpoint, got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("interactive o did not open the Local Endpoint")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("stop Development Session: %v\nstderr:\n%s", err, errorsOutput.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Development Session did not stop after cancellation")
	}

	client := &http.Client{Timeout: 300 * time.Millisecond}
	if response, err := client.Get(endpoint); err == nil {
		response.Body.Close()
		t.Fatal("Local Endpoint still accepted requests after shutdown")
	}
	if after := snapshotFiles(t, projectDirectory); !equalSnapshots(before, after) {
		t.Fatalf("Development Session changed PHP Project files\nbefore: %v\n after: %v", before, after)
	}
}

func TestRunReportsRequiredCapabilityStartupFailure(t *testing.T) {
	projectDirectory := t.TempDir()
	writeTestFile(t, filepath.Join(projectDirectory, "index.php"), "<?php\n")
	project, err := DiscoverProject(projectDirectory)
	if err != nil {
		t.Fatal(err)
	}

	err = Run(context.Background(), Options{
		Project:        project,
		FrankenPHP:     filepath.Join(t.TempDir(), "missing-frankenphp"),
		Environment:    os.Environ(),
		Input:          strings.NewReader(""),
		Output:         io.Discard,
		ErrorOutput:    io.Discard,
		OpenBrowser:    func(string) error { return nil },
		StartupTimeout: time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "start FrankenPHP Required Capability") {
		t.Fatalf("expected actionable Required Capability error, got %v", err)
	}
}

func TestRunCleansUpFrankenPHPWhenStartupTimesOut(t *testing.T) {
	projectDirectory := t.TempDir()
	writeTestFile(t, filepath.Join(projectDirectory, "index.php"), "<?php\n")
	project, err := DiscoverProject(projectDirectory)
	if err != nil {
		t.Fatal(err)
	}
	frankenPHP := buildFakeFrankenPHP(t)

	err = Run(context.Background(), Options{
		Project:    project,
		FrankenPHP: frankenPHP,
		Environment: append(
			os.Environ(),
			"FAKE_FRANKENPHP_NEVER_READY=1",
			"FAKE_FRANKENPHP_IGNORE_TERMINATION=1",
		),
		Input:          strings.NewReader(""),
		Output:         io.Discard,
		ErrorOutput:    io.Discard,
		OpenBrowser:    func(string) error { return nil },
		StartupTimeout: 150 * time.Millisecond,
	})
	if err == nil || !strings.Contains(err.Error(), "was not ready") {
		t.Fatalf("expected actionable startup timeout, got %v", err)
	}
	if err := os.Rename(frankenPHP, frankenPHP+".stopped"); err != nil {
		t.Fatalf("FrankenPHP process still owns its executable after startup cleanup: %v", err)
	}
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (buffer *lockedBuffer) Write(contents []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.b.Write(contents)
}

func (buffer *lockedBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.b.String()
}

func buildFakeFrankenPHP(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	name := "fakefrankenphp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	command := exec.Command("go", "build", "-o", binary, "./tests/testdata/fakefrankenphp")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build fake FrankenPHP: %v\n%s", err, output)
	}
	return binary
}

func waitForEndpoint(t *testing.T, output *lockedBuffer) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, line := range strings.Split(output.String(), "\n") {
			if endpoint, found := strings.CutPrefix(line, "Local Endpoint: "); found {
				return strings.TrimSpace(endpoint)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Local Endpoint was not printed; stdout=%q", output.String())
	return ""
}

func assertResponse(t *testing.T, url, want string) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(contents); got != want {
		t.Fatalf("response mismatch for %s: want %q, got %q", url, want, got)
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		snapshot[relative] = string(contents)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func equalSnapshots(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for path, contents := range left {
		if right[path] != contents {
			return false
		}
	}
	return true
}
