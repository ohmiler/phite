package managedruntime

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func extractZIP(archivePath, destination string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	for _, entry := range archive.File {
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("runtime archive contains symbolic link %q", entry.Name)
		}
		clean, err := cleanArchivePath(entry.Name)
		if err != nil {
			return err
		}
		if clean == verifiedMarkerName {
			return fmt.Errorf("runtime archive contains reserved entry %q", entry.Name)
		}
		target := filepath.Join(destination, clean)
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := extractFile(entry, target); err != nil {
			return fmt.Errorf("extract %q: %w", entry.Name, err)
		}
	}
	return nil
}

func cleanArchivePath(name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("runtime archive entry %q escapes its destination", name)
	}
	return clean, nil
}

func extractFile(entry *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	reader, err := entry.Open()
	if err != nil {
		return err
	}
	mode := entry.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		reader.Close()
		return err
	}
	_, copyErr := io.Copy(output, reader)
	closeOutputErr := output.Close()
	closeReaderErr := reader.Close()
	return errors.Join(copyErr, closeOutputErr, closeReaderErr)
}
