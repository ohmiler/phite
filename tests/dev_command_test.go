package cli_test

import (
	"archive/zip"
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDevCommandStartsConventionalProjectThroughManagedRuntime(t *testing.T) {
	artifact := fakeDevelopmentRuntimeArtifact(t)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(artifact)
	}))
	defer server.Close()

	manifest := writeRuntimeCommandManifest(t, server.URL+"/runtime.zip", bytesSHA256(artifact))
	binary := buildPhiteWithManifest(t, manifest)
	project := t.TempDir()
	public := filepath.Join(project, "public")
	if err := os.MkdirAll(public, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(public, "index.php"), "<?php echo 'front controller';\n")
	writeFile(t, filepath.Join(public, "asset.txt"), "static fixture\n")
	before := projectSnapshot(t, project)

	command := exec.Command(binary, "dev")
	command.Dir = project
	command.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+filepath.Join(t.TempDir(), "runtime-cache"))
	configureInterrupt(command)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatalf("start phite dev: %v", err)
	}

	endpoint := scanEndpoint(t, stdout)
	if !strings.HasPrefix(endpoint, "http://127.0.0.1:") {
		t.Fatalf("Local Endpoint is not loopback-only: %q", endpoint)
	}
	assertHTTPBody(t, endpoint+"/asset.txt", "static fixture\n")
	assertHTTPBody(t, endpoint+"/application/route", "front controller:/application/route")

	if err := interruptProcess(command); err != nil {
		t.Fatalf("interrupt phite dev: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case waitErr := <-done:
		if waitErr != nil {
			t.Fatalf("phite dev did not exit cleanly after Ctrl+C: %v\nstderr:\n%s", waitErr, stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("phite dev did not stop")
	}
	assertEndpointReleased(t, endpoint)
	if after := projectSnapshot(t, project); !sameProjectSnapshot(before, after) {
		t.Fatalf("phite dev changed PHP Project files\nbefore: %v\n after: %v", before, after)
	}
}

func fakeDevelopmentRuntimeArtifact(t *testing.T) []byte {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	phpName := executableName("php")
	frankenPHPName := executableName("frankenphp")
	buildFixtureBinary(t, root, "./tests/testdata/fakephp", filepath.Join(directory, phpName))
	buildFixtureBinary(t, root, "./tests/testdata/fakefrankenphp", filepath.Join(directory, frankenPHPName))

	var artifact bytes.Buffer
	archive := zip.NewWriter(&artifact)
	for _, name := range []string{phpName, frankenPHPName} {
		contents, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.SetMode(0o755)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(contents); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return artifact.Bytes()
}

func executableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func buildFixtureBinary(t *testing.T, root, source, output string) {
	t.Helper()
	command := exec.Command("go", "build", "-o", output, source)
	command.Dir = root
	if contents, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", source, err, contents)
	}
}

func scanEndpoint(t *testing.T, output io.Reader) string {
	t.Helper()
	result := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			if endpoint, ok := strings.CutPrefix(scanner.Text(), "Local Endpoint: "); ok {
				result <- strings.TrimSpace(endpoint)
				return
			}
		}
		result <- ""
	}()
	select {
	case endpoint := <-result:
		if endpoint == "" {
			t.Fatal("phite dev stopped without printing a Local Endpoint")
		}
		return endpoint
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for Local Endpoint")
		return ""
	}
}

func assertHTTPBody(t *testing.T, url, want string) {
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
		t.Fatalf("response mismatch: want %q, got %q", want, got)
	}
}

func assertEndpointReleased(t *testing.T, endpoint string) {
	t.Helper()
	client := &http.Client{Timeout: 300 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(endpoint)
		if err != nil {
			return
		}
		response.Body.Close()
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("Local Endpoint still accepted requests after Ctrl+C")
}

func projectSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[relative] = string(contents)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func sameProjectSnapshot(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for name, contents := range left {
		if right[name] != contents {
			return false
		}
	}
	return true
}
