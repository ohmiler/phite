package cli_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRuntimePackAssemblesVerifiedArtifactAndNotices(t *testing.T) {
	binary := buildRuntimePack(t)
	workspace := t.TempDir()
	artifact := filepath.Join(workspace, "runtime.zip")
	writeZip(t, artifact, map[string]string{
		"license.txt":               "PHP license\n",
		"extras/sbom/php.spdx.json": "{}\n",
		"frankenphp.exe":            "fixture executable\n",
	})
	artifactSHA := fileSHA256(t, artifact)

	writeFile(t, filepath.Join(workspace, "go-licenses.csv"), "example.com/dependency,https://example.com/LICENSE,MIT\n")
	writeFile(t, filepath.Join(workspace, "THIRD-PARTY-NOTICES.md"), "# Notices\n")
	writeFile(t, filepath.Join(workspace, "licenses", "example.com", "dependency", "LICENSE"), "MIT License\n")

	recipePath := filepath.Join(workspace, "recipe.json")
	writeFile(t, recipePath, runtimeRecipe(artifactSHA, `"THIRD-PARTY-NOTICES.md", "licenses/example.com/dependency/LICENSE"`, `"license.txt", "extras/sbom/php.spdx.json"`))
	outputDir := filepath.Join(workspace, "dist")

	command := exec.Command(binary, "assemble", "--recipe", recipePath, "--artifact", artifact, "--output", outputDir)
	command.Dir = workspace
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		t.Fatalf("assemble runtime: %v\nstderr:\n%s", err, stderr.String())
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
	if got := stdout.String(); !strings.Contains(got, "assembled fixture-php-8.5-windows-x64") {
		t.Fatalf("expected assembly confirmation, got %q", got)
	}

	for _, name := range []string{
		"runtime.zip",
		"runtime-manifest.json",
		"third-party-notices-windows-x64.zip",
		"SHA256SUMS",
	} {
		if _, err := os.Stat(filepath.Join(outputDir, name)); err != nil {
			t.Fatalf("expected output %s: %v", name, err)
		}
	}

	manifest, err := os.ReadFile(filepath.Join(outputDir, "runtime-manifest.json"))
	if err != nil {
		t.Fatalf("read runtime manifest: %v", err)
	}
	if !bytes.Contains(manifest, []byte(`"id": "fixture-php-8.5-windows-x64"`)) {
		t.Fatalf("runtime manifest does not contain the Runtime Identity:\n%s", manifest)
	}
	if !bytes.Contains(manifest, []byte(artifactSHA)) {
		t.Fatalf("runtime manifest does not contain the artifact checksum:\n%s", manifest)
	}
}

func TestRuntimePackRejectsInventoryWithoutBundledLicense(t *testing.T) {
	binary := buildRuntimePack(t)
	workspace := t.TempDir()
	artifact := filepath.Join(workspace, "runtime.zip")
	writeZip(t, artifact, map[string]string{"license.txt": "PHP license\n"})

	writeFile(t, filepath.Join(workspace, "go-licenses.csv"), "example.com/missing,https://example.com/LICENSE,MIT\n")
	writeFile(t, filepath.Join(workspace, "THIRD-PARTY-NOTICES.md"), "# Notices\n")
	recipePath := filepath.Join(workspace, "recipe.json")
	writeFile(t, recipePath, runtimeRecipe(fileSHA256(t, artifact), `"THIRD-PARTY-NOTICES.md"`, `"license.txt"`))

	command := exec.Command(binary, "assemble", "--recipe", recipePath, "--artifact", artifact, "--output", filepath.Join(workspace, "dist"))
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err == nil {
		t.Fatal("expected assembly to reject an inventory without a bundled license")
	}
	if got := stderr.String(); !strings.Contains(got, "no bundled license files for example.com/missing") {
		t.Fatalf("expected actionable license inventory error, got %q", got)
	}
}

func runtimeRecipe(artifactSHA, noticeFiles, artifactNotices string) string {
	return fmt.Sprintf(`{
  "schema": 1,
  "identity": {
    "id": "fixture-php-8.5-windows-x64",
    "frankenphp_version": "1.12.7",
    "php_version": "8.5.9",
    "caddy_version": "2.11.4",
    "os": "windows",
    "arch": "x64",
    "support": "tier1",
    "extensions": ["pdo_sqlite"]
  },
  "artifact": {
    "name": "runtime.zip",
    "url": "https://example.com/releases/runtime.zip",
    "sha256": "%s"
  },
  "notices": {
    "name": "third-party-notices-windows-x64.zip",
    "url": "https://example.com/releases/third-party-notices-windows-x64.zip",
    "inventory": "go-licenses.csv",
    "files": [%s],
    "from_artifact": [%s]
  }
}
`, artifactSHA, noticeFiles, artifactNotices)
}

func buildRuntimePack(t *testing.T) string {
	t.Helper()

	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}

	name := "phite-runtime-pack"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	binary := filepath.Join(t.TempDir(), name)
	command := exec.Command("go", "build", "-o", binary, "./cmd/phite-runtime-pack")
	command.Dir = projectRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build phite-runtime-pack: %v\n%s", err, output)
	}
	return binary
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	archive := zip.NewWriter(file)
	for name, contents := range files {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}
