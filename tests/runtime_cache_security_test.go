package cli_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPHPCommandRejectsManifestPathsOutsideRuntimeCache(t *testing.T) {
	artifact := fakeRuntimeArtifact(t)
	artifactSHA := bytesSHA256(artifact)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = response.Write(artifact)
	}))
	defer server.Close()

	testCases := []struct {
		name         string
		identityID   string
		artifactName string
		noticesName  string
		wantError    string
	}{
		{
			name:         "identity traversal",
			identityID:   "../escaped",
			artifactName: "runtime.zip",
			noticesName:  "notices.zip",
			wantError:    "safe lowercase slug",
		},
		{
			name:         "artifact dot segment",
			identityID:   runtimeFixtureIdentity(),
			artifactName: "..",
			noticesName:  "notices.zip",
			wantError:    "artifact.name",
		},
		{
			name:         "notices traversal",
			identityID:   runtimeFixtureIdentity(),
			artifactName: "runtime.zip",
			noticesName:  "../notices.zip",
			wantError:    "notices.name",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			cache := filepath.Join(root, "cache")
			sentinel := filepath.Join(root, "escaped", artifactSHA, "runtime", "sentinel.txt")
			writeFile(t, sentinel, "must survive\n")
			manifest := writeCustomRuntimeManifest(
				t,
				testCase.identityID,
				testCase.artifactName,
				server.URL+"/runtime.zip",
				artifactSHA,
				testCase.noticesName,
				"https://example.com/notices.zip",
			)
			binary := buildPhiteWithManifest(t, manifest)
			command := exec.Command(binary, "php")
			command.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+cache)
			output, err := command.CombinedOutput()
			if err == nil {
				t.Fatalf("expected unsafe manifest path to fail, output=%s", output)
			}
			if !strings.Contains(string(output), testCase.wantError) {
				t.Fatalf("expected error containing %q, got %q", testCase.wantError, output)
			}
			if contents, err := os.ReadFile(sentinel); err != nil || string(contents) != "must survive\n" {
				t.Fatalf("manifest path escaped Runtime Cache and changed sentinel: contents=%q err=%v", contents, err)
			}
		})
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("invalid manifests reached the download server %d times", got)
	}
}

func TestPHPCommandSerializesConcurrentRuntimeAcquisition(t *testing.T) {
	artifact := fakeRuntimeArtifact(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		time.Sleep(150 * time.Millisecond)
		_, _ = response.Write(artifact)
	}))
	defer server.Close()

	cache := filepath.Join(t.TempDir(), "developer-cache")
	manifest := writeRuntimeCommandManifest(t, server.URL+"/runtime.zip", bytesSHA256(artifact))
	binary := buildPhiteWithManifest(t, manifest)

	const invocationCount = 6
	start := make(chan struct{})
	results := make(chan string, invocationCount)
	var wait sync.WaitGroup
	for range invocationCount {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			command := exec.Command(binary, "php")
			command.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+cache)
			output, err := command.CombinedOutput()
			if err != nil {
				results <- fmt.Sprintf("%v\n%s", err, output)
				return
			}
			results <- ""
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	for result := range results {
		if result != "" {
			t.Fatalf("concurrent PHP invocation failed:\n%s", result)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("expected one serialized runtime download, got %d", got)
	}
}

func TestPHPCommandRebuildsTamperedExtractionFromVerifiedArtifact(t *testing.T) {
	artifact := fakeRuntimeArtifact(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = response.Write(artifact)
	}))

	cache := filepath.Join(t.TempDir(), "developer-cache")
	manifest := writeRuntimeCommandManifest(t, server.URL+"/runtime.zip", bytesSHA256(artifact))
	binary := buildPhiteWithManifest(t, manifest)
	first := exec.Command(binary, "php")
	first.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+cache)
	if output, err := first.CombinedOutput(); err != nil {
		t.Fatalf("initial PHP invocation failed: %v\n%s", err, output)
	}
	server.Close()

	phpPath := findCachedPHP(t, cache)
	if err := os.WriteFile(phpPath, []byte("tampered executable\n"), 0o755); err != nil {
		t.Fatalf("tamper extracted PHP fixture: %v", err)
	}

	second := exec.Command(binary, "php")
	second.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+cache)
	var stdout bytes.Buffer
	second.Stdout = &stdout
	if err := second.Run(); err != nil {
		t.Fatalf("runtime was not rebuilt from the verified cached artifact: %v", err)
	}
	if !strings.Contains(stdout.String(), "args=[]") {
		t.Fatalf("rebuilt PHP did not execute expected fixture, stdout=%q", stdout.String())
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("rebuilding extraction unexpectedly downloaded the artifact again; requests=%d", got)
	}
}

func TestPHPCommandRejectsInsecureRedirect(t *testing.T) {
	artifact := fakeRuntimeArtifact(t)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, "http://example.com/runtime.zip", http.StatusFound)
	}))
	defer server.Close()

	cache := filepath.Join(t.TempDir(), "developer-cache")
	manifest := writeRuntimeCommandManifest(t, server.URL+"/runtime.zip", bytesSHA256(artifact))
	binary := buildPhiteWithManifest(t, manifest)
	command := exec.Command(binary, "php")
	command.Env = append(os.Environ(), "PHITE_RUNTIME_CACHE="+cache)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("expected insecure redirect to fail, output=%s", output)
	}
	if !strings.Contains(string(output), "URL must use HTTPS") {
		t.Fatalf("expected redirect policy error, got %q", output)
	}
	assertCacheHasNoVerifiedArtifact(t, cache)
}

func findCachedPHP(t *testing.T, cache string) string {
	t.Helper()
	name := "php"
	if runtimeManifestOS() == "windows" {
		name += ".exe"
	}
	var found string
	err := filepath.WalkDir(cache, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && entry.Name() == name {
			found = path
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect Runtime Cache: %v", err)
	}
	if found == "" {
		t.Fatalf("cached PHP executable %s was not found", name)
	}
	return found
}

func writeCustomRuntimeManifest(
	t *testing.T,
	identityID string,
	artifactName string,
	artifactURL string,
	artifactSHA string,
	noticesName string,
	noticesURL string,
) string {
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
        "name": %q,
        "url": %q,
        "sha256": %q
      },
      "notices": {
        "name": %q,
        "url": %q,
        "sha256": %q
      }
    }
  ]
}
`,
		identityID,
		runtimeManifestOS(),
		runtimeManifestArch(),
		artifactName,
		artifactURL,
		artifactSHA,
		noticesName,
		noticesURL,
		strings.Repeat("1", 64),
	)
	writeFile(t, path, contents)
	return path
}
