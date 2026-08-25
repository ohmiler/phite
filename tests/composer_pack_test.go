package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestComposerPackAssemblesVerifiedArtifactAndNotices(t *testing.T) {
	binary := buildComposerPack(t)
	workspace := composerPackFixture(t)
	output := filepath.Join(workspace, "dist")
	command := exec.Command(
		binary,
		"assemble",
		"--recipe", filepath.Join(workspace, "recipe.json"),
		"--artifact", filepath.Join(workspace, "composer.phar"),
		"--output", output,
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("assemble Composer artifact: %v\nstderr:\n%s", err, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "assembled Composer 2.10.2") {
		t.Fatalf("expected assembly confirmation, got %q", got)
	}
	for _, name := range []string{
		"composer.phar",
		"composer-manifest.json",
		"third-party-notices-composer-2.10.2.zip",
		"SHA256SUMS",
	} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Fatalf("expected output %s: %v", name, err)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(output, "composer-manifest.json"))
	if err != nil {
		t.Fatalf("read Composer Manifest: %v", err)
	}
	if !bytes.Contains(manifest, []byte(`"version": "2.10.2"`)) ||
		!bytes.Contains(manifest, []byte(fileSHA256(t, filepath.Join(workspace, "composer.phar")))) {
		t.Fatalf("Composer Manifest lacks pinned identity:\n%s", manifest)
	}
}

func TestComposerPackRejectsMissingPackageLicense(t *testing.T) {
	binary := buildComposerPack(t)
	workspace := composerPackFixture(t)
	if err := os.Remove(filepath.Join(workspace, "licenses", "example", "dependency", "LICENSE")); err != nil {
		t.Fatalf("remove fixture package license: %v", err)
	}

	command := exec.Command(
		binary,
		"assemble",
		"--recipe", filepath.Join(workspace, "recipe.json"),
		"--artifact", filepath.Join(workspace, "composer.phar"),
		"--output", filepath.Join(workspace, "dist"),
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing package license to fail, output=%s", output)
	}
	if !strings.Contains(string(output), "example/dependency") ||
		!strings.Contains(string(output), "license") {
		t.Fatalf("expected actionable package license error, got %q", output)
	}
}

func TestComposerPackRejectsArtifactChecksumMismatch(t *testing.T) {
	binary := buildComposerPack(t)
	workspace := composerPackFixture(t)
	if err := os.WriteFile(filepath.Join(workspace, "composer.phar"), []byte("altered PHAR\n"), 0o644); err != nil {
		t.Fatalf("alter fixture PHAR: %v", err)
	}

	command := exec.Command(
		binary,
		"assemble",
		"--recipe", filepath.Join(workspace, "recipe.json"),
		"--artifact", filepath.Join(workspace, "composer.phar"),
		"--output", filepath.Join(workspace, "dist"),
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("expected altered Composer PHAR to fail, output=%s", output)
	}
	if !strings.Contains(string(output), "checksum mismatch") {
		t.Fatalf("expected checksum error, got %q", output)
	}
}

func composerPackFixture(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	artifact := filepath.Join(workspace, "composer.phar")
	writeFile(t, artifact, "fixture Composer PHAR\n")
	writeFile(t, filepath.Join(workspace, "inventory.json"), `{
  "packages": [{
    "name": "example/dependency",
    "version": "1.0.0",
    "source": {"reference": "0123456789abcdef"},
    "license": ["MIT"]
  }]
}
`)
	writeFile(t, filepath.Join(workspace, "THIRD-PARTY-NOTICES.md"), "# Composer notices\n")
	writeFile(t, filepath.Join(workspace, "source.json"), "{}\n")
	writeFile(t, filepath.Join(workspace, "licenses", "composer", "LICENSE"), "MIT License\n")
	writeFile(t, filepath.Join(workspace, "licenses", "example", "dependency", "LICENSE"), "MIT License\n")
	recipe := fmt.Sprintf(`{
  "schema": 1,
  "version": "2.10.2",
  "artifact": {
    "name": "composer.phar",
    "url": "https://example.com/releases/composer.phar",
    "sha256": %q
  },
  "notices": {
    "name": "third-party-notices-composer-2.10.2.zip",
    "url": "https://example.com/releases/third-party-notices-composer-2.10.2.zip",
    "inventory": "inventory.json",
    "files": [
      "THIRD-PARTY-NOTICES.md",
      "source.json",
      "licenses/composer/LICENSE",
      "licenses/example/dependency/LICENSE"
    ]
  }
}
`, fileSHA256(t, artifact))
	writeFile(t, filepath.Join(workspace, "recipe.json"), recipe)
	return workspace
}

func buildComposerPack(t *testing.T) string {
	t.Helper()
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	name := "phite-composer-pack"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	command := exec.Command("go", "build", "-o", binary, "./cmd/phite-composer-pack")
	command.Dir = projectRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build phite-composer-pack: %v\n%s", err, output)
	}
	return binary
}
