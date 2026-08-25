package cli_test

import (
	"path/filepath"
	"testing"
)

func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("canonicalize test path %s: %v", path, err)
	}
	return canonical
}
