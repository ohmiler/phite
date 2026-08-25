package devsession

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverProjectRejectsDocumentRootSymlinkOutsideProject(t *testing.T) {
	projectDirectory := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(outside, "index.php"), "<?php\n")
	if err := os.Symlink(outside, filepath.Join(projectDirectory, "linked")); err != nil {
		t.Skipf("create directory symlink: %v", err)
	}
	writeTestFile(t, filepath.Join(projectDirectory, "phite.yaml"), "schema: 1\ndocument_root: linked\n")

	_, err := DiscoverProject(projectDirectory)
	if err == nil || !strings.Contains(err.Error(), "must stay within the PHP Project after resolving symbolic links") {
		t.Fatalf("expected symbolic-link containment error, got %v", err)
	}
}

func TestDiscoverProjectRejectsDuplicateKeysAndMultipleDocuments(t *testing.T) {
	for _, test := range []struct {
		name          string
		configuration string
	}{
		{name: "duplicate key", configuration: "schema: 1\nschema: 1\n"},
		{name: "multiple documents", configuration: "schema: 1\n---\nschema: 1\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			projectDirectory := t.TempDir()
			writeTestFile(t, filepath.Join(projectDirectory, "phite.yaml"), test.configuration)
			_, err := DiscoverProject(projectDirectory)
			if err == nil || !strings.Contains(err.Error(), "parse phite.yaml") {
				t.Fatalf("expected strict structural error, got %v", err)
			}
		})
	}
}

func TestDiscoverProjectRejectsConfigurationSymlinkOutsideProject(t *testing.T) {
	projectDirectory := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	writeTestFile(t, outside, "schema: 1\ndocument_root: public\n")
	if err := os.Symlink(outside, filepath.Join(projectDirectory, "phite.yaml")); err != nil {
		t.Skipf("create configuration symlink: %v", err)
	}

	_, err := DiscoverProject(projectDirectory)
	if err == nil || !strings.Contains(err.Error(), "phite.yaml must stay within the PHP Project") {
		t.Fatalf("expected Project Configuration containment error, got %v", err)
	}
}

func TestDiscoverProjectRejectsNonRegularAndOversizedConfiguration(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		projectDirectory := t.TempDir()
		if err := os.Mkdir(filepath.Join(projectDirectory, "phite.yaml"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := DiscoverProject(projectDirectory)
		if err == nil || !strings.Contains(err.Error(), "phite.yaml is not a regular file") {
			t.Fatalf("expected regular-file error, got %v", err)
		}
	})

	t.Run("oversized", func(t *testing.T) {
		projectDirectory := t.TempDir()
		contents := "schema: 1\n#" + strings.Repeat("x", maxProjectConfigurationSize) + "\n"
		writeTestFile(t, filepath.Join(projectDirectory, "phite.yaml"), contents)
		_, err := DiscoverProject(projectDirectory)
		if err == nil || !strings.Contains(err.Error(), "phite.yaml exceeds") {
			t.Fatalf("expected size-limit error, got %v", err)
		}
	})
}
