package managedcomposer

import (
	"fmt"
	"strings"
	"testing"
)

func TestManifestRejectsUnsafeOrIncompleteComposerIdentity(t *testing.T) {
	testCases := []struct {
		name         string
		version      string
		artifactName string
		noticesName  string
		artifactURL  string
	}{
		{name: "version path separator", version: `2/10`, artifactName: "composer.phar", noticesName: "notices.zip", artifactURL: "https://example.com/composer.phar"},
		{name: "DOS device artifact", version: "2.10.2", artifactName: "NUL.phar", noticesName: "notices.zip", artifactURL: "https://example.com/composer.phar"},
		{name: "alternate data stream", version: "2.10.2", artifactName: "composer.phar:stream", noticesName: "notices.zip", artifactURL: "https://example.com/composer.phar"},
		{name: "notices traversal", version: "2.10.2", artifactName: "composer.phar", noticesName: "../notices.zip", artifactURL: "https://example.com/composer.phar"},
		{name: "insecure remote URL", version: "2.10.2", artifactName: "composer.phar", noticesName: "notices.zip", artifactURL: "http://example.com/composer.phar"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := New(testComposerManifest(testCase.version, testCase.artifactName, testCase.noticesName, testCase.artifactURL), t.TempDir())
			if err == nil {
				t.Fatal("expected invalid Composer Manifest to be rejected")
			}
		})
	}
}

func TestDefaultCacheRootRejectsRelativeOverride(t *testing.T) {
	t.Setenv("PHITE_COMPOSER_CACHE", "relative-cache")
	if _, err := DefaultCacheRoot(); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("expected relative Composer Cache override to fail, got %v", err)
	}
}

func testComposerManifest(version, artifactName, noticesName, artifactURL string) []byte {
	return []byte(fmt.Sprintf(`{
  "schema": 1,
  "composer": {
    "version": %q,
    "artifact": {"name": %q, "url": %q, "sha256": %q},
    "notices": {"name": %q, "url": "https://example.com/notices.zip", "sha256": %q}
  }
}
`, version, artifactName, artifactURL, strings.Repeat("0", 64), noticesName, strings.Repeat("1", 64)))
}
