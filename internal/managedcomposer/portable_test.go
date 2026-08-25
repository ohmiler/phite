package managedcomposer

import "testing"

func TestManifestRejectsEveryNonPortableComposerNameClass(t *testing.T) {
	testCases := []struct {
		name         string
		version      string
		artifactName string
	}{
		{name: "version trailing dot", version: "2.10.2.", artifactName: "composer.phar"},
		{name: "question mark", version: "2.10.2", artifactName: "composer?.phar"},
		{name: "asterisk", version: "2.10.2", artifactName: "composer*.phar"},
		{name: "quote", version: "2.10.2", artifactName: `composer".phar`},
		{name: "angle bracket", version: "2.10.2", artifactName: "composer<.phar"},
		{name: "pipe", version: "2.10.2", artifactName: "composer|.phar"},
		{name: "control", version: "2.10.2", artifactName: "composer\x01.phar"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := New(testComposerManifest(testCase.version, testCase.artifactName, "notices.zip", "https://example.com/composer.phar"), t.TempDir())
			if err == nil {
				t.Fatal("expected non-portable Composer identity to be rejected")
			}
		})
	}
}
