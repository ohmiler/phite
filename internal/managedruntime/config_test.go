package managedruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPHPConfigurationRequiresEveryExplicitSharedLibrary(t *testing.T) {
	runtimeDirectory := t.TempDir()
	if err := os.Mkdir(filepath.Join(runtimeDirectory, "ext"), 0o755); err != nil {
		t.Fatalf("create extension directory: %v", err)
	}
	manager := &Manager{runtime: runtimeSpec{Identity: Identity{OS: "windows", Extensions: []string{"Core", "openssl"}}}}
	if _, _, err := manager.phpConfiguration(runtimeDirectory); err == nil || !strings.Contains(err.Error(), "php_openssl.dll") {
		t.Fatalf("expected missing explicit OpenSSL library to fail, got %v", err)
	}
}

func TestPHPConfigurationUsesExplicitSharedLibraryDirective(t *testing.T) {
	runtimeDirectory := t.TempDir()
	extensionDirectory := filepath.Join(runtimeDirectory, "ext")
	if err := os.Mkdir(extensionDirectory, 0o755); err != nil {
		t.Fatalf("create extension directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(extensionDirectory, "php_openssl.dll"), []byte("fixture"), 0o644); err != nil {
		t.Fatalf("write extension fixture: %v", err)
	}
	manager := &Manager{runtime: runtimeSpec{Identity: Identity{OS: "windows", Extensions: []string{"Core", "openssl"}}}}
	configuration, _, err := manager.phpConfiguration(runtimeDirectory)
	if err != nil {
		t.Fatalf("build PHP configuration: %v", err)
	}
	if !strings.Contains(string(configuration), "extension=php_openssl.dll\n") {
		t.Fatalf("configuration lacks explicit OpenSSL load rule:\n%s", configuration)
	}
}
