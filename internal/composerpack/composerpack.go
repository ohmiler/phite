package composerpack

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	sha256Pattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	packagePattern = regexp.MustCompile(`^[a-z0-9_.-]+/[a-z0-9_.-]+$`)
)

type recipe struct {
	Schema   int      `json:"schema"`
	Version  string   `json:"version"`
	Artifact artifact `json:"artifact"`
	Notices  notices  `json:"notices"`
}

type artifact struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type notices struct {
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Inventory string   `json:"inventory"`
	Files     []string `json:"files"`
}

type inventory struct {
	Packages []inventoryPackage `json:"packages"`
}

type inventoryPackage struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Source  source   `json:"source"`
	License []string `json:"license"`
}

type source struct {
	Reference string `json:"reference"`
}

type manifest struct {
	Schema   int            `json:"schema"`
	Composer manifestRecord `json:"composer"`
}

type manifestRecord struct {
	Version  string        `json:"version"`
	Artifact manifestAsset `json:"artifact"`
	Notices  manifestAsset `json:"notices"`
}

type manifestAsset struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type noticeEntry struct {
	Name string
	Data []byte
}

func Assemble(recipePath, artifactPath, outputDirectory string) (string, error) {
	configuration, err := readRecipe(recipePath)
	if err != nil {
		return "", err
	}
	if err := validateRecipe(configuration); err != nil {
		return "", err
	}
	actualArtifactSHA, err := fileSHA256(artifactPath)
	if err != nil {
		return "", fmt.Errorf("hash Composer artifact: %w", err)
	}
	if actualArtifactSHA != configuration.Artifact.SHA256 {
		return "", fmt.Errorf("Composer artifact checksum mismatch: expected %s, got %s", configuration.Artifact.SHA256, actualArtifactSHA)
	}
	entries, err := collectNoticeEntries(filepath.Dir(recipePath), configuration.Notices)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		return "", fmt.Errorf("create Composer output directory: %w", err)
	}
	artifactOutput := filepath.Join(outputDirectory, configuration.Artifact.Name)
	if err := copyFile(artifactPath, artifactOutput); err != nil {
		return "", fmt.Errorf("copy Composer artifact: %w", err)
	}
	noticesOutput := filepath.Join(outputDirectory, configuration.Notices.Name)
	if err := writeNoticeBundle(noticesOutput, entries); err != nil {
		return "", err
	}
	noticesSHA, err := fileSHA256(noticesOutput)
	if err != nil {
		return "", fmt.Errorf("hash Composer notice bundle: %w", err)
	}
	releaseManifest := manifest{
		Schema: 1,
		Composer: manifestRecord{
			Version: configuration.Version,
			Artifact: manifestAsset{
				Name: configuration.Artifact.Name, URL: configuration.Artifact.URL, SHA256: configuration.Artifact.SHA256,
			},
			Notices: manifestAsset{
				Name: configuration.Notices.Name, URL: configuration.Notices.URL, SHA256: noticesSHA,
			},
		},
	}
	manifestOutput := filepath.Join(outputDirectory, "composer-manifest.json")
	if err := writeJSON(manifestOutput, releaseManifest); err != nil {
		return "", err
	}
	if err := writeChecksums(filepath.Join(outputDirectory, "SHA256SUMS"), []string{artifactOutput, manifestOutput, noticesOutput}); err != nil {
		return "", err
	}
	return configuration.Version, nil
}

func readRecipe(path string) (recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return recipe{}, fmt.Errorf("read Composer recipe: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var configuration recipe
	if err := decoder.Decode(&configuration); err != nil {
		return recipe{}, fmt.Errorf("decode Composer recipe: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return recipe{}, errors.New("Composer recipe contains trailing JSON values")
		}
		return recipe{}, fmt.Errorf("decode Composer recipe: %w", err)
	}
	return configuration, nil
}

func validateRecipe(configuration recipe) error {
	if err := validatePortableRecipeIdentity(configuration); err != nil {
		return err
	}
	if configuration.Schema != 1 {
		return fmt.Errorf("unsupported Composer recipe schema %d", configuration.Schema)
	}
	fields := map[string]string{
		"version": configuration.Version, "artifact.name": configuration.Artifact.Name,
		"artifact.url": configuration.Artifact.URL, "artifact.sha256": configuration.Artifact.SHA256,
		"notices.name": configuration.Notices.Name, "notices.url": configuration.Notices.URL,
		"notices.inventory": configuration.Notices.Inventory,
	}
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("Composer recipe is missing %s", name)
		}
	}
	if !sha256Pattern.MatchString(configuration.Artifact.SHA256) {
		return errors.New("Composer recipe artifact SHA-256 is invalid")
	}
	for name, raw := range map[string]string{
		"artifact.url": configuration.Artifact.URL,
		"notices.url":  configuration.Notices.URL,
	} {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return fmt.Errorf("Composer recipe %s must use HTTPS", name)
		}
	}
	if len(configuration.Notices.Files) == 0 {
		return errors.New("Composer recipe notices.files is empty")
	}
	return nil
}

func collectNoticeEntries(recipeDirectory string, configuration notices) ([]noticeEntry, error) {
	paths := append([]string{configuration.Inventory}, configuration.Files...)
	seen := make(map[string]struct{}, len(paths))
	entries := make([]noticeEntry, 0, len(paths))
	for _, relative := range paths {
		clean, full, err := containedPath(recipeDirectory, relative)
		if err != nil {
			return nil, fmt.Errorf("Composer notice file %q: %w", relative, err)
		}
		if _, duplicate := seen[clean]; duplicate {
			return nil, fmt.Errorf("duplicate Composer notice file %q", relative)
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("read Composer notice file %q: %w", relative, err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("Composer notice file %q is empty", relative)
		}
		seen[clean] = struct{}{}
		entries = append(entries, noticeEntry{Name: filepath.ToSlash(clean), Data: data})
	}
	packages, err := parseInventory(entries[0].Data)
	if err != nil {
		return nil, err
	}
	if _, ok := seen[filepath.Clean("licenses/composer/LICENSE")]; !ok {
		return nil, errors.New("Composer root license is missing from the notice bundle")
	}
	for _, pkg := range packages {
		prefix := filepath.ToSlash(filepath.Clean("licenses/"+pkg.Name)) + "/"
		found := false
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name, prefix) {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("Composer package %s has no bundled license", pkg.Name)
		}
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name < entries[right].Name })
	return entries, nil
}

func parseInventory(data []byte) ([]inventoryPackage, error) {
	var installed inventory
	if err := json.Unmarshal(data, &installed); err != nil {
		return nil, fmt.Errorf("decode Composer package inventory: %w", err)
	}
	if len(installed.Packages) == 0 {
		return nil, errors.New("Composer package inventory is empty")
	}
	seen := make(map[string]struct{}, len(installed.Packages))
	for _, pkg := range installed.Packages {
		if !packagePattern.MatchString(pkg.Name) || pkg.Version == "" || pkg.Source.Reference == "" || len(pkg.License) == 0 {
			return nil, fmt.Errorf("Composer package inventory has incomplete metadata for %s", pkg.Name)
		}
		if _, duplicate := seen[pkg.Name]; duplicate {
			return nil, fmt.Errorf("Composer package inventory duplicates %s", pkg.Name)
		}
		seen[pkg.Name] = struct{}{}
		for _, license := range pkg.License {
			if strings.TrimSpace(license) == "" || strings.EqualFold(license, "unknown") {
				return nil, fmt.Errorf("Composer package inventory has unknown license for %s", pkg.Name)
			}
		}
	}
	return installed.Packages, nil
}

func containedPath(root, relative string) (string, string, error) {
	if filepath.IsAbs(relative) {
		return "", "", errors.New("path must be relative")
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", errors.New("path must stay inside the recipe directory")
	}
	return clean, filepath.Join(root, clean), nil
}

func writeNoticeBundle(path string, entries []noticeEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create Composer notice bundle: %w", err)
	}
	archive := zip.NewWriter(file)
	fixedTime := time.Unix(0, 0).UTC()
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.Name, Method: zip.Deflate}
		header.SetModTime(fixedTime)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			archive.Close()
			file.Close()
			return fmt.Errorf("create Composer notice entry %q: %w", entry.Name, err)
		}
		if _, err := writer.Write(entry.Data); err != nil {
			archive.Close()
			file.Close()
			return fmt.Errorf("write Composer notice entry %q: %w", entry.Name, err)
		}
	}
	return errors.Join(archive.Close(), file.Close())
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	return errors.Join(copyErr, output.Close())
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func writeChecksums(path string, files []string) error {
	sort.Slice(files, func(left, right int) bool { return filepath.Base(files[left]) < filepath.Base(files[right]) })
	var output strings.Builder
	for _, file := range files {
		digest, err := fileSHA256(file)
		if err != nil {
			return err
		}
		fmt.Fprintf(&output, "%s  %s\n", digest, filepath.Base(file))
	}
	return os.WriteFile(path, []byte(output.String()), 0o644)
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
