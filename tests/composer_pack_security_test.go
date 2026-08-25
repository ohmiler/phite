package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposerPackRejectsNonPortableReleaseAssetNames(t *testing.T) {
	testCases := []struct {
		name        string
		original    string
		replacement string
	}{
		{name: "DOS device", original: `"name": "composer.phar"`, replacement: `"name": "NUL.phar"`},
		{name: "alternate data stream", original: `"name": "composer.phar"`, replacement: `"name": "composer.phar:stream"`},
		{name: "wildcard", original: `"name": "composer.phar"`, replacement: `"name": "composer*.phar"`},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			binary := buildComposerPack(t)
			workspace := composerPackFixture(t)
			recipePath := filepath.Join(workspace, "recipe.json")
			data, err := os.ReadFile(recipePath)
			if err != nil {
				t.Fatalf("read fixture recipe: %v", err)
			}
			data = []byte(strings.Replace(string(data), testCase.original, testCase.replacement, 1))
			if err := os.WriteFile(recipePath, data, 0o644); err != nil {
				t.Fatalf("write fixture recipe: %v", err)
			}
			command := exec.Command(binary, "assemble", "--recipe", recipePath, "--artifact", filepath.Join(workspace, "composer.phar"), "--output", filepath.Join(workspace, "dist"))
			output, err := command.CombinedOutput()
			if err == nil {
				t.Fatalf("expected non-portable release asset to fail, output=%s", output)
			}
			if !strings.Contains(string(output), "portable file name") {
				t.Fatalf("expected portable name error, got %q", output)
			}
		})
	}
}
