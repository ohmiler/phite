package runtimepack

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type recipe struct {
	Schema   int      `json:"schema"`
	Identity identity `json:"identity"`
	Artifact artifact `json:"artifact"`
	Notices  notices  `json:"notices"`
}

type identity struct {
	ID                string   `json:"id"`
	FrankenPHPVersion string   `json:"frankenphp_version"`
	PHPVersion        string   `json:"php_version"`
	CaddyVersion      string   `json:"caddy_version"`
	OS                string   `json:"os"`
	Arch              string   `json:"arch"`
	Support           string   `json:"support"`
	Extensions        []string `json:"extensions"`
}

type artifact struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type notices struct {
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	Inventory    string   `json:"inventory"`
	Files        []string `json:"files"`
	FromArtifact []string `json:"from_artifact"`
}

type runtimeManifest struct {
	Schema   int               `json:"schema"`
	Runtimes []manifestRuntime `json:"runtimes"`
}

type manifestRuntime struct {
	Identity identity       `json:"identity"`
	Artifact artifact       `json:"artifact"`
	Notices  manifestNotice `json:"notices"`
}

type manifestNotice struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type noticeEntry struct {
	Name string
	Data []byte
}

func Assemble(recipePath, artifactPath, outputDir string) (string, error) {
	recipe, err := readRecipe(recipePath)
	if err != nil {
		return "", err
	}
	if err := validateRecipe(recipe); err != nil {
		return "", err
	}

	recipeDir, err := filepath.Abs(filepath.Dir(recipePath))
	if err != nil {
		return "", fmt.Errorf("resolve recipe directory: %w", err)
	}
	entries, err := collectNoticeEntries(recipeDir, artifactPath, recipe.Notices)
	if err != nil {
		return "", err
	}

	artifactDigest, err := fileSHA256(artifactPath)
	if err != nil {
		return "", fmt.Errorf("hash runtime artifact: %w", err)
	}
	if artifactDigest != strings.ToLower(recipe.Artifact.SHA256) {
		return "", fmt.Errorf("runtime artifact checksum mismatch: expected %s, got %s", recipe.Artifact.SHA256, artifactDigest)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}
	artifactOutput := filepath.Join(outputDir, recipe.Artifact.Name)
	if err := copyFile(artifactPath, artifactOutput); err != nil {
		return "", fmt.Errorf("copy runtime artifact: %w", err)
	}

	noticesOutput := filepath.Join(outputDir, recipe.Notices.Name)
	if err := writeNoticeBundle(noticesOutput, entries); err != nil {
		return "", err
	}
	noticesDigest, err := fileSHA256(noticesOutput)
	if err != nil {
		return "", fmt.Errorf("hash notice bundle: %w", err)
	}

	manifest := runtimeManifest{
		Schema: recipe.Schema,
		Runtimes: []manifestRuntime{{
			Identity: recipe.Identity,
			Artifact: recipe.Artifact,
			Notices: manifestNotice{
				Name:   recipe.Notices.Name,
				URL:    recipe.Notices.URL,
				SHA256: noticesDigest,
			},
		}},
	}
	manifestOutput := filepath.Join(outputDir, "runtime-manifest.json")
	if err := writeJSON(manifestOutput, manifest); err != nil {
		return "", err
	}

	if err := writeChecksums(filepath.Join(outputDir, "SHA256SUMS"), []string{
		artifactOutput,
		manifestOutput,
		noticesOutput,
	}); err != nil {
		return "", err
	}

	return recipe.Identity.ID, nil
}

func readRecipe(path string) (recipe, error) {
	file, err := os.Open(path)
	if err != nil {
		return recipe{}, fmt.Errorf("open recipe: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var parsed recipe
	if err := decoder.Decode(&parsed); err != nil {
		return recipe{}, fmt.Errorf("decode recipe: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return recipe{}, errors.New("decode recipe: unexpected trailing content")
	}
	return parsed, nil
}

func validateRecipe(recipe recipe) error {
	if recipe.Schema != 1 {
		return fmt.Errorf("recipe schema must be 1, got %d", recipe.Schema)
	}
	required := map[string]string{
		"identity.id":                 recipe.Identity.ID,
		"identity.frankenphp_version": recipe.Identity.FrankenPHPVersion,
		"identity.php_version":        recipe.Identity.PHPVersion,
		"identity.caddy_version":      recipe.Identity.CaddyVersion,
		"identity.os":                 recipe.Identity.OS,
		"identity.arch":               recipe.Identity.Arch,
		"identity.support":            recipe.Identity.Support,
		"artifact.name":               recipe.Artifact.Name,
		"artifact.url":                recipe.Artifact.URL,
		"artifact.sha256":             recipe.Artifact.SHA256,
		"notices.name":                recipe.Notices.Name,
		"notices.url":                 recipe.Notices.URL,
		"notices.inventory":           recipe.Notices.Inventory,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("recipe field %s is required", field)
		}
	}
	if recipe.Identity.Support != "tier1" && recipe.Identity.Support != "experimental" {
		return fmt.Errorf("identity.support must be tier1 or experimental, got %q", recipe.Identity.Support)
	}
	if len(recipe.Identity.Extensions) == 0 {
		return errors.New("identity.extensions must contain at least one extension")
	}
	if err := validateLeafName("artifact.name", recipe.Artifact.Name); err != nil {
		return err
	}
	if err := validateLeafName("notices.name", recipe.Notices.Name); err != nil {
		return err
	}
	if err := validateHTTPSURL("artifact.url", recipe.Artifact.URL); err != nil {
		return err
	}
	if err := validateHTTPSURL("notices.url", recipe.Notices.URL); err != nil {
		return err
	}
	digest, err := hex.DecodeString(recipe.Artifact.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return errors.New("artifact.sha256 must be a 64-character SHA-256 digest")
	}
	if len(recipe.Notices.Files) == 0 {
		return errors.New("notices.files must contain at least one notice file")
	}
	if len(recipe.Notices.FromArtifact) == 0 {
		return errors.New("notices.from_artifact must contain at least one artifact notice")
	}
	return nil
}

func validateLeafName(field, value string) error {
	if filepath.Base(value) != value || value == "." || value == ".." {
		return fmt.Errorf("%s must be a file name without directories", field)
	}
	return nil
}

func validateHTTPSURL(field, value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute HTTPS URL", field)
	}
	return nil
}

func collectNoticeEntries(recipeDir, artifactPath string, notices notices) ([]noticeEntry, error) {
	paths := append([]string{notices.Inventory}, notices.Files...)
	seen := make(map[string]struct{}, len(paths)+len(notices.FromArtifact))
	entries := make([]noticeEntry, 0, len(paths)+len(notices.FromArtifact))
	var inventoryPackages []string
	for _, relative := range paths {
		clean, full, err := containedPath(recipeDir, relative)
		if err != nil {
			return nil, fmt.Errorf("notice file %q: %w", relative, err)
		}
		if _, duplicate := seen[clean]; duplicate {
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("read notice file %q: %w", relative, err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("notice file %q is empty", relative)
		}
		if relative == notices.Inventory {
			if err := validateInventory(data); err != nil {
				return nil, err
			}
			inventoryPackages = inventoryPackageNames(data)
		}
		seen[clean] = struct{}{}
		entries = append(entries, noticeEntry{Name: filepath.ToSlash(clean), Data: data})
	}
	if err := validateBundledLicenses(inventoryPackages, entries); err != nil {
		return nil, err
	}

	archive, err := zip.OpenReader(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("open runtime artifact as ZIP: %w", err)
	}
	defer archive.Close()
	artifactEntries := make(map[string]*zip.File, len(archive.File))
	for _, file := range archive.File {
		artifactEntries[file.Name] = file
	}
	for _, name := range notices.FromArtifact {
		file, ok := artifactEntries[name]
		if !ok {
			return nil, fmt.Errorf("runtime artifact is missing required notice %q", name)
		}
		reader, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open artifact notice %q: %w", name, err)
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read artifact notice %q: %w", name, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close artifact notice %q: %w", name, closeErr)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("artifact notice %q is empty", name)
		}
		entryName := "artifact/" + name
		if _, duplicate := seen[entryName]; duplicate {
			return nil, fmt.Errorf("duplicate notice entry %q", entryName)
		}
		seen[entryName] = struct{}{}
		entries = append(entries, noticeEntry{Name: entryName, Data: data})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

func containedPath(root, relative string) (string, string, error) {
	if filepath.IsAbs(relative) {
		return "", "", errors.New("path must be relative")
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", errors.New("path must stay inside the recipe directory")
	}
	full := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", errors.New("path must stay inside the recipe directory")
	}
	return clean, full, nil
}

func validateInventory(data []byte) error {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.FieldsPerRecord = 3
	rows, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("parse notice inventory: %w", err)
	}
	if len(rows) == 0 {
		return errors.New("notice inventory is empty")
	}
	for index, row := range rows {
		for _, field := range row {
			if strings.TrimSpace(field) == "" {
				return fmt.Errorf("notice inventory row %d has an empty field", index+1)
			}
		}
		if strings.EqualFold(row[1], "unknown") || strings.EqualFold(row[2], "unknown") {
			return fmt.Errorf("notice inventory row %d has an unknown license record for %s", index+1, row[0])
		}
	}
	return nil
}

func writeNoticeBundle(path string, entries []noticeEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create notice bundle: %w", err)
	}
	archive := zip.NewWriter(file)
	fixedTime := time.Unix(0, 0).UTC()
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.Name, Method: zip.Deflate}
		header.SetModTime(fixedTime)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("create notice entry %q: %w", entry.Name, err)
		}
		if _, err := writer.Write(entry.Data); err != nil {
			return fmt.Errorf("write notice entry %q: %w", entry.Name, err)
		}
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close notice bundle: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close notice bundle file: %w", err)
	}
	return nil
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
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode runtime manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write runtime manifest: %w", err)
	}
	return nil
}

func writeChecksums(path string, files []string) error {
	sort.Slice(files, func(i, j int) bool { return filepath.Base(files[i]) < filepath.Base(files[j]) })
	var contents strings.Builder
	for _, file := range files {
		digest, err := fileSHA256(file)
		if err != nil {
			return fmt.Errorf("hash release asset %q: %w", filepath.Base(file), err)
		}
		fmt.Fprintf(&contents, "%s  %s\n", digest, filepath.Base(file))
	}
	if err := os.WriteFile(path, []byte(contents.String()), 0o644); err != nil {
		return fmt.Errorf("write checksums: %w", err)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
