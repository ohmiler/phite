package managedruntime

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type contentEntry struct {
	name string
	size uint64
	open func() (io.ReadCloser, error)
}

func extractionMatchesArtifact(artifactPath, extractedDirectory string) (bool, error) {
	expected, err := archiveContentDigest(artifactPath)
	if err != nil {
		return false, err
	}
	actual, err := directoryContentDigest(extractedDirectory)
	if err != nil {
		return false, err
	}
	return expected == actual, nil
}

func archiveContentDigest(path string) ([sha256.Size]byte, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer archive.Close()

	entries := make([]contentEntry, 0, len(archive.File))
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return [sha256.Size]byte{}, fmt.Errorf("runtime archive contains symbolic link %q", file.Name)
		}
		clean, err := cleanArchivePath(file.Name)
		if err != nil {
			return [sha256.Size]byte{}, err
		}
		if clean == verifiedMarkerName {
			return [sha256.Size]byte{}, fmt.Errorf("runtime archive contains reserved entry %q", file.Name)
		}
		file := file
		entries = append(entries, contentEntry{
			name: filepath.ToSlash(clean),
			size: file.UncompressedSize64,
			open: file.Open,
		})
	}
	return digestEntries(entries)
}

func directoryContentDigest(root string) ([sha256.Size]byte, error) {
	var entries []contentEntry
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if filepath.ToSlash(relative) == verifiedMarkerName {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("extracted runtime contains non-regular file %q", relative)
		}
		entryPath := path
		entries = append(entries, contentEntry{
			name: filepath.ToSlash(relative),
			size: uint64(info.Size()),
			open: func() (io.ReadCloser, error) {
				return os.Open(entryPath)
			},
		})
		return nil
	})
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return digestEntries(entries)
}

func digestEntries(entries []contentEntry) ([sha256.Size]byte, error) {
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].name < entries[right].name
	})
	hasher := sha256.New()
	for _, entry := range entries {
		if err := writeContentEntry(hasher, entry); err != nil {
			return [sha256.Size]byte{}, err
		}
	}
	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))
	return digest, nil
}

func writeContentEntry(hasher hash.Hash, entry contentEntry) error {
	if err := binary.Write(hasher, binary.BigEndian, uint64(len(entry.name))); err != nil {
		return err
	}
	if _, err := io.WriteString(hasher, entry.name); err != nil {
		return err
	}
	if err := binary.Write(hasher, binary.BigEndian, entry.size); err != nil {
		return err
	}
	reader, err := entry.open()
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(hasher, reader)
	closeErr := reader.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if uint64(written) != entry.size {
		return errors.New("runtime content size changed while hashing")
	}
	return nil
}
