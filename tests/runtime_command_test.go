package cli_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPHPCommandDownloadsVerifiesCachesAndPreservesProcessContract(t *testing.T) {
	artifact := fakeRuntimeArtifact(t)
	artifactSHA := bytesSHA256(artifact)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(artifact)
	}))

	cache := filepath.Join(t.TempDir(), "developer-cache")
	manifest := writeRuntimeCommandManifest(t, server.URL+"/runtime.zip", artifactSHA)
	binary := buildPhiteWithManifest(t, manifest)
	project := t.TempDir()

	command := exec.Command(binary, "php", "script with spaces.php", "--exit=7")
	command.Dir = project
	command.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+cache)
	command.Stdin = strings.NewReader("input from developer\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	exitError, ok := err.(*exec.ExitError)
	if !ok || exitError.ExitCode() != 7 {
		t.Fatalf("expected PHP exit status 7, got %v\nstderr:\n%s", err, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "cwd="+project+"\n") ||
		!strings.Contains(got, `args=["script with spaces.php","--exit=7"]`) ||
		!strings.Contains(got, "stdin=input from developer\n") {
		t.Fatalf("PHP stdout did not preserve cwd, arguments, and stdin:\n%s", got)
	}
	if got, want := stderr.String(), "fake php stderr\n"; got != want {
		t.Fatalf("PHP stderr mismatch\nwant: %q\n got: %q", want, got)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("expected one first-use download, got %d", got)
	}

	server.Close()
	offline := exec.Command(binary, "php")
	offline.Dir = project
	offline.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+cache)
	if output, err := offline.CombinedOutput(); err != nil {
		t.Fatalf("cached PHP command failed offline: %v\n%s", err, output)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("offline invocation attempted another download; requests=%d", got)
	}

	versionCommand := exec.Command(binary, "version")
	versionCommand.Dir = project
	versionCommand.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+cache)
	versionOutput, err := versionCommand.CombinedOutput()
	if err != nil {
		t.Fatalf("run phite version after installation: %v\n%s", err, versionOutput)
	}
	expectedIdentity := runtimeFixtureIdentity()
	if got, want := string(versionOutput), "Phite CLI dev\nManaged Runtime "+expectedIdentity+"\n"; got != want {
		t.Fatalf("version output mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestPHPCommandNeverExtractsOrExecutesChecksumMismatch(t *testing.T) {
	artifact := fakeRuntimeArtifact(t)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(artifact)
	}))
	defer server.Close()

	cache := filepath.Join(t.TempDir(), "developer-cache")
	manifest := writeRuntimeCommandManifest(t, server.URL+"/runtime.zip", strings.Repeat("0", 64))
	binary := buildPhiteWithManifest(t, manifest)
	command := exec.Command(binary, "php")
	command.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+cache)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err == nil {
		t.Fatal("expected checksum mismatch to fail")
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("checksum mismatch executed PHP, stdout=%q", got)
	}
	if got := stderr.String(); !strings.Contains(got, "checksum mismatch") {
		t.Fatalf("expected actionable checksum error, got %q", got)
	}
	assertCacheHasNoVerifiedArtifact(t, cache)
}

func TestPHPCommandRecoversAfterInterruptedDownload(t *testing.T) {
	artifact := fakeRuntimeArtifact(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			response.Header().Set("Content-Length", strconv.Itoa(len(artifact)))
			response.WriteHeader(http.StatusOK)
			_, _ = response.Write(artifact[:len(artifact)/2])
			return
		}
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(artifact)
	}))
	defer server.Close()

	cache := filepath.Join(t.TempDir(), "developer-cache")
	manifest := writeRuntimeCommandManifest(t, server.URL+"/runtime.zip", bytesSHA256(artifact))
	binary := buildPhiteWithManifest(t, manifest)

	first := exec.Command(binary, "php")
	first.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+cache)
	if output, err := first.CombinedOutput(); err == nil {
		t.Fatalf("expected interrupted download to fail, output=%s", output)
	}
	assertCacheHasNoVerifiedArtifact(t, cache)

	second := exec.Command(binary, "php")
	second.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+cache)
	if output, err := second.CombinedOutput(); err != nil {
		t.Fatalf("retry after interrupted download failed: %v\n%s", err, output)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("expected interrupted download to be retried, requests=%d", got)
	}
}

func buildPhiteWithManifest(t *testing.T, manifestPath string) string {
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
	command := exec.Command(
		"go",
		"build",
		"-ldflags",
		"-X main.runtimeManifestPath="+manifestPath,
		"-o",
		binary,
		"./cmd/phite",
	)
	command.Dir = projectRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build phite with fixture manifest: %v\n%s", err, output)
	}
	return binary
}

func fakeRuntimeArtifact(t *testing.T) []byte {
	t.Helper()
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	executableName := "php"
	if runtime.GOOS == "windows" {
		executableName += ".exe"
	}
	executablePath := filepath.Join(t.TempDir(), executableName)
	command := exec.Command("go", "build", "-o", executablePath, "./tests/testdata/fakephp")
	command.Dir = projectRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build fake PHP: %v\n%s", err, output)
	}
	executable, err := os.ReadFile(executablePath)
	if err != nil {
		t.Fatalf("read fake PHP: %v", err)
	}

	var artifact bytes.Buffer
	archive := zip.NewWriter(&artifact)
	header := &zip.FileHeader{Name: executableName, Method: zip.Store}
	header.SetMode(0o755)
	writer, err := archive.CreateHeader(header)
	if err != nil {
		t.Fatalf("create fake PHP ZIP entry: %v", err)
	}
	if _, err := writer.Write(executable); err != nil {
		t.Fatalf("write fake PHP ZIP entry: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close fake runtime artifact: %v", err)
	}
	return artifact.Bytes()
}

func writeRuntimeCommandManifest(t *testing.T, artifactURL, artifactSHA string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime-manifest.json")
	contents := fmt.Sprintf(`{
  "schema": 1,
  "runtimes": [
    {
      "identity": {
        "id": %q,
        "frankenphp_version": "fixture",
        "php_version": "8.5.0",
        "caddy_version": "fixture",
        "os": %q,
        "arch": %q,
        "support": "tier1",
        "extensions": ["pdo_sqlite"]
      },
      "artifact": {
        "name": "runtime.zip",
        "url": %q,
        "sha256": %q
      },
      "notices": {
        "name": "notices.zip",
        "url": "https://example.com/notices.zip",
        "sha256": %q
      }
    }
  ]
}
`, runtimeFixtureIdentity(), runtimeManifestOS(), runtimeManifestArch(), artifactURL, artifactSHA, strings.Repeat("1", 64))
	writeFile(t, path, contents)
	return path
}

func runtimeFixtureIdentity() string {
	return "fixture-php-8.5-" + runtimeManifestOS() + "-" + runtimeManifestArch()
}

func runtimeManifestOS() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return runtime.GOOS
}

func runtimeManifestArch() string {
	if runtime.GOARCH == "amd64" {
		return "x64"
	}
	return runtime.GOARCH
}

func bytesSHA256(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}

func assertCacheHasNoVerifiedArtifact(t *testing.T, cache string) {
	t.Helper()
	err := filepath.WalkDir(cache, func(path string, entry os.DirEntry, walkErr error) error {
		if os.IsNotExist(walkErr) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.Name() == "verified.json" || entry.Name() == "runtime.zip" {
			t.Errorf("failed download left verified cache state at %s", path)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("inspect runtime cache: %v", err)
	}
}
