package devsession

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverProjectChecksConventionalEntrypointsInOrder(t *testing.T) {
	for _, entrypoint := range []string{"public/index.php", "web/index.php", "index.php"} {
		t.Run(entrypoint, func(t *testing.T) {
			projectDirectory := t.TempDir()
			writeTestFile(t, filepath.Join(projectDirectory, filepath.FromSlash(entrypoint)), "<?php\n")

			project, err := DiscoverProject(projectDirectory)
			if err != nil {
				t.Fatalf("discover %s: %v", entrypoint, err)
			}
			if got, want := project.Entrypoint, filepath.Join(projectDirectory, filepath.FromSlash(entrypoint)); got != want {
				t.Fatalf("Entrypoint mismatch: want %q, got %q", want, got)
			}
		})
	}
}

func TestDiscoverProjectUsesSchemaOneDocumentRootOverride(t *testing.T) {
	projectDirectory := t.TempDir()
	entrypoint := filepath.Join(projectDirectory, "application", "http", "index.php")
	writeTestFile(t, entrypoint, "<?php\n")
	writeTestFile(t, filepath.Join(projectDirectory, "public", "index.php"), "<?php echo 'must not be selected';\n")
	configuration := []byte("schema: 1\ndocument_root: application/http\n")
	configurationPath := filepath.Join(projectDirectory, "phite.yaml")
	if err := os.WriteFile(configurationPath, configuration, 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := DiscoverProject(projectDirectory)
	if err != nil {
		t.Fatalf("discover overridden Project: %v", err)
	}
	if got, want := project.DocumentRoot, filepath.Dir(entrypoint); got != want {
		t.Fatalf("Document Root mismatch: want %q, got %q", want, got)
	}
	if got, want := project.Entrypoint, entrypoint; got != want {
		t.Fatalf("Entrypoint mismatch: want %q, got %q", want, got)
	}
	after, err := os.ReadFile(configurationPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(after); got != string(configuration) {
		t.Fatalf("validation rewrote Project Configuration\nwant: %q\n got: %q", configuration, after)
	}
}

func TestDiscoverProjectRejectsMissingAndAmbiguousEntrypointsWithExample(t *testing.T) {
	for _, test := range []struct {
		name  string
		files []string
		want  string
	}{
		{name: "missing", want: "no conventional Entrypoint"},
		{name: "ambiguous", files: []string{"public/index.php", "web/index.php"}, want: "ambiguous Entrypoint"},
	} {
		t.Run(test.name, func(t *testing.T) {
			projectDirectory := t.TempDir()
			for _, file := range test.files {
				writeTestFile(t, filepath.Join(projectDirectory, filepath.FromSlash(file)), "<?php\n")
			}
			_, err := DiscoverProject(projectDirectory)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q error, got %v", test.want, err)
			}
			for _, example := range []string{"phite.yaml", "schema: 1", "document_root:"} {
				if !strings.Contains(err.Error(), example) {
					t.Fatalf("error lacks configuration example %q: %v", example, err)
				}
			}
		})
	}
}

func TestDiscoverProjectRejectsInvalidProjectConfiguration(t *testing.T) {
	for _, test := range []struct {
		name          string
		configuration string
		prepare       func(*testing.T, string)
		want          string
	}{
		{name: "malformed", configuration: "schema: [\n", want: "parse phite.yaml"},
		{name: "missing schema", configuration: "document_root: public\n", want: "schema is required"},
		{name: "unsupported schema", configuration: "schema: 2\n", want: "unsupported Configuration Schema 2"},
		{name: "unknown key", configuration: "schema: 1\ndocument_rooot: public\n", want: "field document_rooot not found"},
		{name: "invalid schema type", configuration: "schema: one\n", want: "Configuration Schema must be an integer"},
		{name: "invalid Document Root type", configuration: "schema: 1\ndocument_root: 42\n", want: "Document Root must be a string"},
		{name: "empty Document Root", configuration: "schema: 1\ndocument_root: '  '\n", want: "Document Root is empty"},
		{name: "absolute Document Root", configuration: "schema: 1\ndocument_root: /outside\n", want: "must be relative"},
		{name: "backslash Document Root", configuration: "schema: 1\ndocument_root: public\\site\n", want: "forward slashes"},
		{name: "escaping Document Root", configuration: "schema: 1\ndocument_root: ../outside\n", want: "must stay within"},
		{name: "missing Document Root", configuration: "schema: 1\ndocument_root: missing\n", want: "does not exist"},
		{
			name: "Document Root is file", configuration: "schema: 1\ndocument_root: target\n", want: "is not a directory",
			prepare: func(t *testing.T, project string) { writeTestFile(t, filepath.Join(project, "target"), "file\n") },
		},
		{
			name: "override lacks Entrypoint", configuration: "schema: 1\ndocument_root: target\n", want: "does not contain index.php",
			prepare: func(t *testing.T, project string) {
				if err := os.Mkdir(filepath.Join(project, "target"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			projectDirectory := t.TempDir()
			if test.prepare != nil {
				test.prepare(t, projectDirectory)
			}
			configurationPath := filepath.Join(projectDirectory, "phite.yaml")
			if err := os.WriteFile(configurationPath, []byte(test.configuration), 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := DiscoverProject(projectDirectory)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
			after, readErr := os.ReadFile(configurationPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if got := string(after); got != test.configuration {
				t.Fatalf("validation changed invalid Project Configuration\nwant: %q\n got: %q", test.configuration, got)
			}
		})
	}
}
