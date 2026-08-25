package managedcomposer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Manager struct {
	composer  composerSpec
	directory string
	client    *http.Client
}

type Installation struct {
	Version string
	PHAR    string
}

func New(manifestData []byte, cacheRoot string) (*Manager, error) {
	spec, err := parseManifest(manifestData)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cacheRoot) == "" {
		return nil, errors.New("Composer Cache path is empty")
	}
	absoluteCache, err := filepath.Abs(cacheRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve Composer Cache: %w", err)
	}
	directory := filepath.Join(absoluteCache, spec.Version, spec.Artifact.SHA256)
	relative, err := filepath.Rel(absoluteCache, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, errors.New("Composer version resolves outside the Composer Cache")
	}
	return &Manager{
		composer:  spec,
		directory: directory,
		client: &http.Client{
			Timeout: 10 * time.Minute,
			CheckRedirect: func(request *http.Request, previous []*http.Request) error {
				if len(previous) >= 10 {
					return errors.New("stopped after 10 redirects")
				}
				return validateArtifactURL(request.URL.String())
			},
		},
	}, nil
}

func DefaultCacheRoot() (string, error) {
	if override := os.Getenv("PHITE_COMPOSER_CACHE"); override != "" {
		if !filepath.IsAbs(override) {
			return "", errors.New("PHITE_COMPOSER_CACHE must be an absolute path")
		}
		return filepath.Clean(override), nil
	}
	userCache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate Developer cache directory: %w", err)
	}
	return filepath.Join(userCache, "phite", "composer"), nil
}

func (manager *Manager) Acquire(ctx context.Context) (Installation, error) {
	if err := os.MkdirAll(manager.directory, 0o700); err != nil {
		return Installation{}, fmt.Errorf("create Composer Cache: %w", err)
	}
	lock, err := acquireComposerLock(filepath.Join(manager.directory, ".acquire.lock"))
	if err != nil {
		return Installation{}, fmt.Errorf("lock Composer Cache: %w", err)
	}
	defer lock.Close()

	artifactPath := filepath.Join(manager.directory, manager.composer.Artifact.Name)
	if err := manager.ensureArtifact(ctx, artifactPath); err != nil {
		return Installation{}, err
	}
	return Installation{Version: manager.composer.Version, PHAR: artifactPath}, nil
}

func (manager *Manager) ensureArtifact(ctx context.Context, artifactPath string) error {
	digest, err := fileSHA256(artifactPath)
	switch {
	case err == nil && digest == manager.composer.Artifact.SHA256:
		return nil
	case err == nil:
		if removeErr := os.Remove(artifactPath); removeErr != nil {
			return fmt.Errorf("remove corrupt cached Composer artifact: %w", removeErr)
		}
	case !os.IsNotExist(err):
		return fmt.Errorf("inspect cached Composer artifact: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, manager.composer.Artifact.URL, nil)
	if err != nil {
		return fmt.Errorf("create Composer download request: %w", err)
	}
	request.Header.Set("User-Agent", "Phite-CLI")
	response, err := manager.client.Do(request)
	if err != nil {
		return fmt.Errorf("download Composer: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download Composer: unexpected HTTP status %s", response.Status)
	}

	temporary, err := os.CreateTemp(filepath.Dir(artifactPath), ".composer-download-*")
	if err != nil {
		return fmt.Errorf("create temporary Composer download: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	hasher := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(temporary, hasher), response.Body)
	syncErr := temporary.Sync()
	closeErr := temporary.Close()
	if copyErr != nil {
		return fmt.Errorf("download Composer: %w", copyErr)
	}
	if syncErr != nil {
		return fmt.Errorf("flush Composer download: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close Composer download: %w", closeErr)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != manager.composer.Artifact.SHA256 {
		return fmt.Errorf("Composer artifact checksum mismatch: expected %s, got %s", manager.composer.Artifact.SHA256, actual)
	}
	if err := os.Rename(temporaryPath, artifactPath); err != nil {
		return fmt.Errorf("publish verified Composer artifact: %w", err)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
