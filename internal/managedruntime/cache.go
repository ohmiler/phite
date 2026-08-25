package managedruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

const verifiedMarkerName = "verified.json"

type Manager struct {
	runtime   runtimeSpec
	directory string
	client    *http.Client
}

type Installation struct {
	Identity Identity
	PHP      string
}

type verifiedMarker struct {
	IdentityID     string `json:"identity_id"`
	ArtifactSHA256 string `json:"artifact_sha256"`
}

func New(manifestData []byte, cacheRoot string) (*Manager, error) {
	selected, err := parseCurrentRuntime(manifestData)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cacheRoot) == "" {
		return nil, errors.New("Runtime Cache path is empty")
	}
	absoluteCache, err := filepath.Abs(cacheRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve Runtime Cache: %w", err)
	}
	directory := filepath.Join(absoluteCache, selected.Identity.ID, selected.Artifact.SHA256)
	relative, err := filepath.Rel(absoluteCache, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, errors.New("Runtime Identity resolves outside the Runtime Cache")
	}

	return &Manager{
		runtime:   selected,
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
	if override := os.Getenv("PHITE_RUNTIME_CACHE"); override != "" {
		if !filepath.IsAbs(override) {
			return "", errors.New("PHITE_RUNTIME_CACHE must be an absolute path")
		}
		return filepath.Clean(override), nil
	}
	userCache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate Developer cache directory: %w", err)
	}
	return filepath.Join(userCache, "phite", "runtimes"), nil
}

func (manager *Manager) Acquire(ctx context.Context) (Installation, error) {
	if err := os.MkdirAll(manager.directory, 0o700); err != nil {
		return Installation{}, fmt.Errorf("create Runtime Cache: %w", err)
	}
	lock, err := acquireFileLock(filepath.Join(manager.directory, ".acquire.lock"))
	if err != nil {
		return Installation{}, fmt.Errorf("lock Runtime Cache: %w", err)
	}
	defer lock.Close()

	artifactPath := filepath.Join(manager.directory, manager.runtime.Artifact.Name)
	if err := manager.ensureArtifact(ctx, artifactPath); err != nil {
		return Installation{}, err
	}

	phpPath, err := manager.ensureExtracted(artifactPath)
	if err != nil {
		return Installation{}, err
	}
	return Installation{Identity: manager.runtime.Identity, PHP: phpPath}, nil
}

func (manager *Manager) Installed() (Identity, bool, error) {
	if _, err := os.Stat(manager.directory); os.IsNotExist(err) {
		return Identity{}, false, nil
	} else if err != nil {
		return Identity{}, false, fmt.Errorf("inspect Runtime Cache: %w", err)
	}
	lock, err := acquireFileLock(filepath.Join(manager.directory, ".acquire.lock"))
	if err != nil {
		return Identity{}, false, fmt.Errorf("lock Runtime Cache: %w", err)
	}
	defer lock.Close()

	artifactPath := filepath.Join(manager.directory, manager.runtime.Artifact.Name)
	digest, err := fileSHA256(artifactPath)
	if os.IsNotExist(err) {
		return Identity{}, false, nil
	}
	if err != nil {
		return Identity{}, false, fmt.Errorf("verify cached runtime artifact: %w", err)
	}
	if digest != manager.runtime.Artifact.SHA256 {
		return Identity{}, false, nil
	}
	valid, err := manager.validExtraction(artifactPath)
	if err != nil {
		return Identity{}, false, err
	}
	if !valid {
		return Identity{}, false, nil
	}
	return manager.runtime.Identity, true, nil
}

func (manager *Manager) ensureArtifact(ctx context.Context, artifactPath string) error {
	digest, err := fileSHA256(artifactPath)
	switch {
	case err == nil && digest == manager.runtime.Artifact.SHA256:
		return nil
	case err == nil:
		if removeErr := os.Remove(artifactPath); removeErr != nil {
			return fmt.Errorf("remove corrupt cached runtime artifact: %w", removeErr)
		}
	case !os.IsNotExist(err):
		return fmt.Errorf("inspect cached runtime artifact: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, manager.runtime.Artifact.URL, nil)
	if err != nil {
		return fmt.Errorf("create runtime download request: %w", err)
	}
	request.Header.Set("User-Agent", "Phite-CLI")
	response, err := manager.client.Do(request)
	if err != nil {
		return fmt.Errorf("download Managed Runtime: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download Managed Runtime: unexpected HTTP status %s", response.Status)
	}

	temporary, err := os.CreateTemp(filepath.Dir(artifactPath), ".runtime-download-*")
	if err != nil {
		return fmt.Errorf("create temporary runtime download: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	hasher := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(temporary, hasher), response.Body)
	syncErr := temporary.Sync()
	closeErr := temporary.Close()
	if copyErr != nil {
		return fmt.Errorf("download Managed Runtime: %w", copyErr)
	}
	if syncErr != nil {
		return fmt.Errorf("flush runtime download: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close runtime download: %w", closeErr)
	}

	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != manager.runtime.Artifact.SHA256 {
		return fmt.Errorf("runtime artifact checksum mismatch: expected %s, got %s", manager.runtime.Artifact.SHA256, actual)
	}
	if err := os.Rename(temporaryPath, artifactPath); err != nil {
		return fmt.Errorf("publish verified runtime artifact: %w", err)
	}
	return nil
}

func (manager *Manager) ensureExtracted(artifactPath string) (string, error) {
	valid, err := manager.validExtraction(artifactPath)
	if err != nil {
		return "", err
	}
	finalDirectory := filepath.Join(manager.directory, "runtime")
	if valid {
		return manager.phpPath(finalDirectory), nil
	}

	if err := os.RemoveAll(finalDirectory); err != nil {
		return "", fmt.Errorf("remove incomplete runtime extraction: %w", err)
	}
	temporaryDirectory, err := os.MkdirTemp(manager.directory, ".runtime-extract-*")
	if err != nil {
		return "", fmt.Errorf("create temporary runtime extraction: %w", err)
	}
	defer os.RemoveAll(temporaryDirectory)

	if err := extractZIP(artifactPath, temporaryDirectory); err != nil {
		return "", fmt.Errorf("extract Managed Runtime: %w", err)
	}
	phpPath := manager.phpPath(temporaryDirectory)
	if info, err := os.Stat(phpPath); err != nil || info.IsDir() {
		return "", fmt.Errorf("Managed Runtime is missing PHP executable %s", filepath.Base(phpPath))
	}
	matches, err := extractionMatchesArtifact(artifactPath, temporaryDirectory)
	if err != nil {
		return "", fmt.Errorf("verify extracted Managed Runtime: %w", err)
	}
	if !matches {
		return "", errors.New("extracted Managed Runtime differs from its verified artifact")
	}

	markerData, err := json.Marshal(verifiedMarker{
		IdentityID:     manager.runtime.Identity.ID,
		ArtifactSHA256: manager.runtime.Artifact.SHA256,
	})
	if err != nil {
		return "", fmt.Errorf("encode runtime verification marker: %w", err)
	}
	markerData = append(markerData, '\n')
	if err := os.WriteFile(filepath.Join(temporaryDirectory, verifiedMarkerName), markerData, 0o600); err != nil {
		return "", fmt.Errorf("write runtime verification marker: %w", err)
	}
	if err := os.Rename(temporaryDirectory, finalDirectory); err != nil {
		return "", fmt.Errorf("publish verified runtime extraction: %w", err)
	}
	return manager.phpPath(finalDirectory), nil
}

func (manager *Manager) validExtraction(artifactPath string) (bool, error) {
	extracted := filepath.Join(manager.directory, "runtime")
	markerData, err := os.ReadFile(filepath.Join(extracted, verifiedMarkerName))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read runtime verification marker: %w", err)
	}
	var marker verifiedMarker
	if json.Unmarshal(markerData, &marker) != nil {
		return false, nil
	}
	if marker.IdentityID != manager.runtime.Identity.ID || marker.ArtifactSHA256 != manager.runtime.Artifact.SHA256 {
		return false, nil
	}
	info, err := os.Stat(manager.phpPath(extracted))
	if os.IsNotExist(err) || (err == nil && info.IsDir()) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect extracted PHP executable: %w", err)
	}
	matches, err := extractionMatchesArtifact(artifactPath, extracted)
	if err != nil {
		return false, fmt.Errorf("verify extracted Managed Runtime: %w", err)
	}
	return matches, nil
}

func (manager *Manager) phpPath(extractedDirectory string) string {
	name := "php"
	if manager.runtime.Identity.OS == "windows" || goruntime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(extractedDirectory, name)
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
