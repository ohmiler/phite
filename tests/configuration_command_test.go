package cli_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDevCommandUsesConfiguredDocumentRoot(t *testing.T) {
	artifact := fakeDevelopmentRuntimeArtifact(t)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(artifact)
	}))
	defer server.Close()
	manifest := writeRuntimeCommandManifest(t, server.URL+"/runtime.zip", bytesSHA256(artifact))
	binary := buildPhiteWithManifest(t, manifest)
	project := copyProjectFixture(t, "overridden")
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
		t.Fatalf("start configured phite dev: %v", err)
	}

	endpoint := scanEndpoint(t, stdout)
	assertHTTPBody(t, endpoint+"/asset.txt", "overridden static fixture\n")
	assertHTTPBody(t, endpoint+"/configured/route", "front controller:/configured/route")
	if err := interruptProcess(command); err != nil {
		t.Fatalf("interrupt configured phite dev: %v", err)
	}
	waitForDevelopmentCommand(t, command, &stderr)
	assertEndpointReleased(t, endpoint)
	if after := projectSnapshot(t, project); !sameProjectSnapshot(before, after) {
		t.Fatalf("configured Development Session changed PHP Project files\nbefore: %v\n after: %v", before, after)
	}
}

func TestDevCommandRejectsDiscoveryAndConfigurationErrorsBeforeRuntimeAcquisition(t *testing.T) {
	artifact := fakeDevelopmentRuntimeArtifact(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(artifact)
	}))
	defer server.Close()
	manifest := writeRuntimeCommandManifest(t, server.URL+"/runtime.zip", bytesSHA256(artifact))
	binary := buildPhiteWithManifest(t, manifest)

	for _, test := range []struct {
		fixture string
		want    []string
	}{
		{fixture: "missing", want: []string{"no conventional Entrypoint", "Project Configuration example (phite.yaml)", "schema: 1", "document_root:"}},
		{fixture: "ambiguous", want: []string{"ambiguous Entrypoint", "public/index.php", "web/index.php", "Project Configuration example (phite.yaml)"}},
		{fixture: "invalid", want: []string{"parse phite.yaml", "field document_rooot not found"}},
	} {
		t.Run(test.fixture, func(t *testing.T) {
			project := copyProjectFixture(t, test.fixture)
			before := projectSnapshot(t, project)
			command := exec.Command(binary, "dev")
			command.Dir = project
			command.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+filepath.Join(t.TempDir(), "runtime-cache"))
			output, err := command.CombinedOutput()
			if err == nil {
				t.Fatalf("expected %s fixture to fail, output:\n%s", test.fixture, output)
			}
			for _, want := range test.want {
				if !strings.Contains(string(output), want) {
					t.Fatalf("%s error lacks %q:\n%s", test.fixture, want, output)
				}
			}
			if after := projectSnapshot(t, project); !sameProjectSnapshot(before, after) {
				t.Fatalf("validation changed %s fixture\nbefore: %v\n after: %v", test.fixture, before, after)
			}
		})
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("invalid Project discovery acquired a Managed Runtime %d time(s)", got)
	}
}

func waitForDevelopmentCommand(t *testing.T, command *exec.Cmd, stderr *bytes.Buffer) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("phite dev did not exit cleanly after Ctrl+C: %v\nstderr:\n%s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("phite dev did not stop")
	}
}

func copyProjectFixture(t *testing.T, name string) string {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("testdata/projects", name))
	if err != nil {
		t.Fatalf("resolve %s fixture: %v", name, err)
	}
	destination := filepath.Join(t.TempDir(), "project")
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		t.Fatalf("copy %s fixture: %v", name, err)
	}
	return destination
}
