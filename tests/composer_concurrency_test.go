package cli_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestComposerCommandSerializesConcurrentArtifactAcquisition(t *testing.T) {
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
			time.Sleep(150 * time.Millisecond)
			_, _ = response.Write(composerArtifact)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	cacheRoot := t.TempDir()
	runtimeManifest := writeRuntimeCommandManifest(t, server.URL+"/runtime.zip", bytesSHA256(runtimeArtifact))
	composerManifest := writeComposerCommandManifest(t, server.URL+"/composer.phar", bytesSHA256(composerArtifact))
	binary := buildPhiteWithManifests(t, runtimeManifest, composerManifest)
	environment := append(os.Environ(), "PHITE_RUNTIME_CACHE="+filepath.Join(cacheRoot, "runtime"), "PHITE_COMPOSER_CACHE="+filepath.Join(cacheRoot, "composer"))

	const invocationCount = 6
	start := make(chan struct{})
	results := make(chan string, invocationCount)
	var wait sync.WaitGroup
	for range invocationCount {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			command := exec.Command(binary, "composer", "--version")
			command.Env = environment
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
			t.Fatalf("concurrent Composer invocation failed:\n%s", result)
		}
	}
	if got := composerRequests.Load(); got != 1 {
		t.Fatalf("expected one serialized Composer download, got %d", got)
	}
	if got := runtimeRequests.Load(); got != 1 {
		t.Fatalf("expected one serialized Runtime download, got %d", got)
	}
}
