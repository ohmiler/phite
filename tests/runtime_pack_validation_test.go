package cli_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimePackRejectsMissingReleaseInputs(t *testing.T) {
	binary := buildRuntimePack(t)
	workspace := t.TempDir()
	artifact := filepath.Join(workspace, "runtime.zip")
	writeZip(t, artifact, map[string]string{"license.txt": "PHP license\n"})
	artifactSHA := fileSHA256(t, artifact)
	validRecipe := runtimeRecipe(artifactSHA, `"licenses/example.com/dependency/LICENSE"`, `"license.txt"`)

	tests := []struct {
		name       string
		recipe     string
		wantStderr string
	}{
		{
			name:       "checksum",
			recipe:     strings.Replace(validRecipe, `"sha256": "`+artifactSHA+`"`, `"sha256": ""`, 1),
			wantStderr: "artifact.sha256",
		},
		{
			name:       "runtime metadata",
			recipe:     strings.Replace(validRecipe, `"php_version": "8.5.9"`, `"php_version": ""`, 1),
			wantStderr: "identity.php_version",
		},
		{
			name:       "license inventory",
			recipe:     strings.Replace(validRecipe, `"inventory": "go-licenses.csv"`, `"inventory": ""`, 1),
			wantStderr: "notices.inventory",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recipePath := filepath.Join(t.TempDir(), "recipe.json")
			writeFile(t, recipePath, test.recipe)
			command := exec.Command(binary, "assemble", "--recipe", recipePath, "--artifact", artifact, "--output", filepath.Join(t.TempDir(), "dist"))
			var stderr bytes.Buffer
			command.Stderr = &stderr
			if err := command.Run(); err == nil {
				t.Fatalf("expected assembly to reject missing %s", test.name)
			}
			if got := stderr.String(); !strings.Contains(got, test.wantStderr) {
				t.Fatalf("expected error containing %q, got %q", test.wantStderr, got)
			}
		})
	}
}

func TestRuntimePackRejectsAlteredArtifactBeforePublishing(t *testing.T) {
	binary := buildRuntimePack(t)
	workspace := t.TempDir()
	artifact := filepath.Join(workspace, "runtime.zip")
	writeZip(t, artifact, map[string]string{"license.txt": "PHP license\n"})
	writeFile(t, filepath.Join(workspace, "go-licenses.csv"), "example.com/dependency,https://example.com/LICENSE,MIT\n")
	writeFile(t, filepath.Join(workspace, "licenses", "example.com", "dependency", "LICENSE"), "MIT License\n")
	recipePath := filepath.Join(workspace, "recipe.json")
	writeFile(t, recipePath, runtimeRecipe(strings.Repeat("0", 64), `"licenses/example.com/dependency/LICENSE"`, `"license.txt"`))
	outputDir := filepath.Join(workspace, "dist")

	command := exec.Command(binary, "assemble", "--recipe", recipePath, "--artifact", artifact, "--output", outputDir)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err == nil {
		t.Fatal("expected assembly to reject an altered artifact")
	}
	if got := stderr.String(); !strings.Contains(got, "runtime artifact checksum mismatch") {
		t.Fatalf("expected actionable checksum error, got %q", got)
	}
	if _, err := filepath.Glob(filepath.Join(outputDir, "*")); err != nil {
		t.Fatalf("inspect output directory: %v", err)
	} else if matches, _ := filepath.Glob(filepath.Join(outputDir, "*")); len(matches) != 0 {
		t.Fatalf("checksum failure published partial assets: %v", matches)
	}
}
