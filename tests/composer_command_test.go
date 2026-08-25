package cli_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func TestComposerCommandUsesManagedPHPAndVerifiedOfflineArtifact(t *testing.T) {
	runtimeArtifact := fakeRuntimeArtifact(t)
	composerArtifact := []byte("fixture composer phar\n")
	var runtimeRequests atomic.Int32
	var composerRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/runtime.zip":
			runtimeRequests.Add(1)
			_, _ = response.Write(runtimeArtifact)
		case "/composer.phar":
			composerRequests.Add(1)
			_, _ = response.Write(composerArtifact)
		default:
			http.NotFound(response, request)
		}
	}))

	runtimeManifest := writeRuntimeCommandManifest(t, server.URL+"/runtime.zip", bytesSHA256(runtimeArtifact))
	composerManifest := writeComposerCommandManifest(t, server.URL+"/composer.phar", bytesSHA256(composerArtifact))
	binary := buildPhiteWithManifests(t, runtimeManifest, composerManifest)
	cacheRoot := t.TempDir()
	project := canonicalPath(t, t.TempDir())

	command := exec.Command(binary, "composer", "install", "--exit=9")
	command.Dir = project
	command.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+filepath.Join(cacheRoot, "runtime"), "PHITE_COMPOSER_CACHE="+filepath.Join(cacheRoot, "composer"))
	command.Stdin = strings.NewReader("composer stdin\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	exitError, ok := err.(*exec.ExitError)
	if !ok || exitError.ExitCode() != 9 {
		t.Fatalf("expected Composer exit status 9, got %v\nstderr:\n%s", err, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "cwd="+project+"\n") || !strings.Contains(got, `"install","--exit=9"]`) || !strings.Contains(got, "stdin=composer stdin\n") || !strings.Contains(got, "composer.phar") {
		t.Fatalf("Composer process contract was not preserved:\n%s", got)
	}
	if got, want := stderr.String(), "fake php stderr\n"; got != want {
		t.Fatalf("Composer stderr mismatch\nwant: %q\n got: %q", want, got)
	}
	if runtimeRequests.Load() != 1 || composerRequests.Load() != 1 {
		t.Fatalf("expected one runtime and Composer download, got runtime=%d composer=%d", runtimeRequests.Load(), composerRequests.Load())
	}

	server.Close()
	offline := exec.Command(binary, "composer", "--version")
	offline.Dir = project
	offline.Env = command.Env
	if output, err := offline.CombinedOutput(); err != nil {
		t.Fatalf("cached Composer command failed offline: %v\n%s", err, output)
	}
	if runtimeRequests.Load() != 1 || composerRequests.Load() != 1 {
		t.Fatalf("offline Composer invocation attempted a download")
	}

	version := exec.Command(binary, "version")
	version.Env = command.Env
	versionOutput, err := version.CombinedOutput()
	if err != nil {
		t.Fatalf("report version after Composer: %v\n%s", err, versionOutput)
	}
	if !strings.Contains(string(versionOutput), "Managed Runtime "+runtimeFixtureIdentity()) {
		t.Fatalf("Composer changed selected Runtime Identity: %s", versionOutput)
	}
}

func TestComposerCommandRejectsAlteredArtifactBeforeExecution(t *testing.T) {
	runtimeArtifact := fakeRuntimeArtifact(t)
	composerArtifact := []byte("altered composer phar\n")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/runtime.zip" {
			_, _ = response.Write(runtimeArtifact)
			return
		}
		_, _ = response.Write(composerArtifact)
	}))
	defer server.Close()

	runtimeManifest := writeRuntimeCommandManifest(t, server.URL+"/runtime.zip", bytesSHA256(runtimeArtifact))
	composerManifest := writeComposerCommandManifest(t, server.URL+"/composer.phar", strings.Repeat("0", 64))
	binary := buildPhiteWithManifests(t, runtimeManifest, composerManifest)
	command := exec.Command(binary, "composer", "install")
	command.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+filepath.Join(t.TempDir(), "runtime"), "PHITE_COMPOSER_CACHE="+filepath.Join(t.TempDir(), "composer"))
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("expected altered Composer artifact to fail, output=%s", output)
	}
	if strings.Contains(string(output), "cwd=") {
		t.Fatalf("altered Composer artifact executed PHP, output=%q", output)
	}
	if !strings.Contains(string(output), "checksum mismatch") {
		t.Fatalf("expected Composer checksum error, got %q", output)
	}
}

func writeComposerCommandManifest(t *testing.T, artifactURL, artifactSHA string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "composer-manifest.json")
	writeFile(t, path, fmt.Sprintf(`{
  "schema": 1,
  "composer": {
    "version": "2.10.2",
    "artifact": {"name": "composer.phar", "url": %q, "sha256": %q},
    "notices": {"name": "composer-notices.zip", "url": "https://example.com/composer-notices.zip", "sha256": %q}
  }
}
`, artifactURL, artifactSHA, strings.Repeat("1", 64)))
	return path
}

func buildPhiteWithManifests(t *testing.T, runtimeManifest, composerManifest string) string {
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
	command := exec.Command("go", "build", "-ldflags", "-X main.runtimeManifestPath="+runtimeManifest+" -X main.composerManifestPath="+composerManifest, "-o", binary, "./cmd/phite")
	command.Dir = projectRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build phite with fixture manifests: %v\n%s", err, output)
	}
	return binary
}
