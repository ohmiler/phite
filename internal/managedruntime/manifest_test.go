package managedruntime

import (
	"fmt"
	"strings"
	"testing"
)

func TestManifestRejectsNonPortableNamesAndUnknownSupport(t *testing.T) {
	testCases := []struct {
		name         string
		identityID   string
		artifactName string
		noticesName  string
		support      string
	}{
		{name: "identity trailing dot", identityID: "runtime.", artifactName: "runtime.zip", noticesName: "notices.zip", support: "tier1"},
		{name: "DOS device artifact", identityID: "runtime", artifactName: "NUL.zip", noticesName: "notices.zip", support: "tier1"},
		{name: "alternate data stream", identityID: "runtime", artifactName: "runtime.zip:stream", noticesName: "notices.zip", support: "tier1"},
		{name: "artifact trailing dot", identityID: "runtime", artifactName: "runtime.zip.", noticesName: "notices.zip", support: "tier1"},
		{name: "DOS device notices", identityID: "runtime", artifactName: "runtime.zip", noticesName: "con.txt", support: "tier1"},
		{name: "unknown support", identityID: "runtime", artifactName: "runtime.zip", noticesName: "notices.zip", support: "stable"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := New(testManifest(
				testCase.identityID,
				testCase.artifactName,
				testCase.noticesName,
				testCase.support,
			), t.TempDir())
			if err == nil {
				t.Fatal("expected invalid Runtime Manifest to be rejected")
			}
		})
	}
}

func testManifest(identityID, artifactName, noticesName, support string) []byte {
	return []byte(fmt.Sprintf(`{
  "schema": 1,
  "runtimes": [{
    "identity": {
      "id": %q,
      "frankenphp_version": "fixture",
      "php_version": "8.5.0",
      "caddy_version": "fixture",
      "os": %q,
      "arch": %q,
      "support": %q,
      "extensions": ["pdo_sqlite"]
    },
    "artifact": {
      "name": %q,
      "url": "https://example.com/runtime.zip",
      "sha256": %q
    },
    "notices": {
      "name": %q,
      "url": "https://example.com/notices.zip",
      "sha256": %q
    }
  }]
}
`,
		identityID,
		manifestOS(),
		manifestArch(),
		support,
		artifactName,
		strings.Repeat("0", 64),
		noticesName,
		strings.Repeat("1", 64),
	))
}
