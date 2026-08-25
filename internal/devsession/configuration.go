package devsession

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	configurationExample        = "Project Configuration example (phite.yaml):\n  schema: 1\n  document_root: public"
	maxProjectConfigurationSize = 64 * 1024
)

type projectConfiguration struct {
	Schema       schemaField       `yaml:"schema"`
	DocumentRoot documentRootField `yaml:"document_root,omitempty"`
}

type schemaField struct {
	present bool
	value   int
}

func (field *schemaField) UnmarshalYAML(node *yaml.Node) error {
	field.present = true
	if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
		return errors.New("Configuration Schema must be an integer")
	}
	value, err := strconv.Atoi(node.Value)
	if err != nil {
		return fmt.Errorf("Configuration Schema must be an integer: %w", err)
	}
	field.value = value
	return nil
}

type documentRootField struct {
	present bool
	value   string
}

func (field *documentRootField) UnmarshalYAML(node *yaml.Node) error {
	field.present = true
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return errors.New("Document Root must be a string")
	}
	field.value = node.Value
	return nil
}

func discoverProject(directory string) (Project, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return Project{}, fmt.Errorf("resolve PHP Project directory: %w", err)
	}
	canonicalProject, err := filepath.EvalSymlinks(filepath.Clean(absolute))
	if err != nil {
		return Project{}, fmt.Errorf("resolve PHP Project directory: %w", err)
	}
	configuration, err := readProjectConfiguration(canonicalProject)
	if err != nil {
		return Project{}, err
	}
	if configuration != nil && configuration.DocumentRoot.present {
		return discoverConfiguredProject(canonicalProject, configuration.DocumentRoot.value)
	}
	return discoverConventionalProject(canonicalProject)
}

func readProjectConfiguration(projectDirectory string) (*projectConfiguration, error) {
	configurationPath := filepath.Join(projectDirectory, "phite.yaml")
	_, err := os.Lstat(configurationPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect phite.yaml: %w", err)
	}
	canonicalPath, err := filepath.EvalSymlinks(configurationPath)
	if err != nil {
		return nil, fmt.Errorf("resolve phite.yaml: %w", err)
	}
	if !pathWithin(projectDirectory, canonicalPath) {
		return nil, errors.New("validate phite.yaml: phite.yaml must stay within the PHP Project after resolving symbolic links")
	}
	info, err := os.Stat(canonicalPath)
	if err != nil {
		return nil, fmt.Errorf("inspect phite.yaml: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("validate phite.yaml: phite.yaml is not a regular file")
	}
	if info.Size() > maxProjectConfigurationSize {
		return nil, fmt.Errorf("validate phite.yaml: phite.yaml exceeds the %d-byte size limit", maxProjectConfigurationSize)
	}
	file, err := os.Open(canonicalPath)
	if err != nil {
		return nil, fmt.Errorf("read phite.yaml: %w", err)
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, maxProjectConfigurationSize+1))
	if err != nil {
		return nil, fmt.Errorf("read phite.yaml: %w", err)
	}
	if len(contents) > maxProjectConfigurationSize {
		return nil, fmt.Errorf("validate phite.yaml: phite.yaml exceeds the %d-byte size limit", maxProjectConfigurationSize)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	var configuration projectConfiguration
	err = decoder.Decode(&configuration)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	if err != nil {
		return nil, fmt.Errorf("parse phite.yaml: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err == nil {
		return nil, errors.New("parse phite.yaml: multiple YAML documents are not supported")
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse phite.yaml: %w", err)
	}
	if !configuration.Schema.present {
		return nil, errors.New("validate phite.yaml: Configuration Schema schema is required")
	}
	if configuration.Schema.value != 1 {
		return nil, fmt.Errorf("validate phite.yaml: unsupported Configuration Schema %d; Phite supports schema 1", configuration.Schema.value)
	}
	return &configuration, nil
}

func discoverConfiguredProject(projectDirectory, configuredRoot string) (Project, error) {
	trimmed := strings.TrimSpace(configuredRoot)
	if trimmed == "" {
		return Project{}, errors.New("validate phite.yaml: Document Root is empty")
	}
	if trimmed != configuredRoot {
		return Project{}, errors.New("validate phite.yaml: Document Root must not contain surrounding whitespace")
	}
	if strings.ContainsRune(configuredRoot, '\x00') {
		return Project{}, errors.New("validate phite.yaml: Document Root contains a null byte")
	}
	if strings.Contains(configuredRoot, `\`) {
		return Project{}, errors.New("validate phite.yaml: Document Root must use forward slashes")
	}
	if path.IsAbs(configuredRoot) || filepath.IsAbs(configuredRoot) || looksLikeWindowsVolume(configuredRoot) {
		return Project{}, errors.New("validate phite.yaml: Document Root must be relative to the PHP Project")
	}
	cleaned := path.Clean(configuredRoot)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return Project{}, errors.New("validate phite.yaml: Document Root must stay within the PHP Project")
	}
	documentRoot := filepath.Join(projectDirectory, filepath.FromSlash(cleaned))
	info, err := os.Stat(documentRoot)
	if os.IsNotExist(err) {
		return Project{}, fmt.Errorf("validate phite.yaml: Document Root %q does not exist", configuredRoot)
	}
	if err != nil {
		return Project{}, fmt.Errorf("validate phite.yaml: inspect Document Root %q: %w", configuredRoot, err)
	}
	if !info.IsDir() {
		return Project{}, fmt.Errorf("validate phite.yaml: Document Root %q is not a directory", configuredRoot)
	}
	canonicalRoot, err := filepath.EvalSymlinks(documentRoot)
	if err != nil {
		return Project{}, fmt.Errorf("validate phite.yaml: resolve Document Root %q: %w", configuredRoot, err)
	}
	if !pathWithin(projectDirectory, canonicalRoot) {
		return Project{}, errors.New("validate phite.yaml: Document Root must stay within the PHP Project after resolving symbolic links")
	}
	entrypoint := filepath.Join(canonicalRoot, "index.php")
	if err := validateEntrypoint(canonicalRoot, entrypoint); err != nil {
		if os.IsNotExist(err) {
			return Project{}, fmt.Errorf("validate phite.yaml: Document Root %q does not contain index.php", configuredRoot)
		}
		return Project{}, fmt.Errorf("validate phite.yaml: %w", err)
	}
	return Project{Directory: projectDirectory, DocumentRoot: canonicalRoot, Entrypoint: entrypoint}, nil
}

func discoverConventionalProject(projectDirectory string) (Project, error) {
	type candidate struct {
		root    string
		display string
	}
	candidates := []candidate{
		{root: filepath.Join(projectDirectory, "public"), display: "public/index.php"},
		{root: filepath.Join(projectDirectory, "web"), display: "web/index.php"},
		{root: projectDirectory, display: "index.php"},
	}
	var matches []candidate
	for _, candidate := range candidates {
		entrypoint := filepath.Join(candidate.root, "index.php")
		if _, err := os.Stat(entrypoint); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return Project{}, fmt.Errorf("inspect conventional Entrypoint %s: %w", candidate.display, err)
		}
		canonicalRoot, err := filepath.EvalSymlinks(candidate.root)
		if err != nil {
			return Project{}, fmt.Errorf("resolve Document Root for %s: %w", candidate.display, err)
		}
		if !pathWithin(projectDirectory, canonicalRoot) {
			return Project{}, fmt.Errorf("Document Root for %s resolves outside the PHP Project", candidate.display)
		}
		err = validateEntrypoint(canonicalRoot, entrypoint)
		switch {
		case err == nil:
			candidate.root = canonicalRoot
			matches = append(matches, candidate)
		default:
			return Project{}, fmt.Errorf("inspect conventional Entrypoint %s: %w", candidate.display, err)
		}
	}
	if len(matches) == 0 {
		return Project{}, fmt.Errorf("Supported Project has no conventional Entrypoint at public/index.php, web/index.php, or index.php.\n%s", configurationExample)
	}
	if len(matches) > 1 {
		locations := make([]string, 0, len(matches))
		for _, match := range matches {
			locations = append(locations, match.display)
		}
		return Project{}, fmt.Errorf("Supported Project has an ambiguous Entrypoint (%s).\n%s", strings.Join(locations, ", "), configurationExample)
	}
	root := matches[0].root
	return Project{Directory: projectDirectory, DocumentRoot: root, Entrypoint: filepath.Join(root, "index.php")}, nil
}

func validateEntrypoint(containingRoot, entrypoint string) error {
	info, err := os.Stat(entrypoint)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("Entrypoint index.php is not a regular file")
	}
	canonicalEntrypoint, err := filepath.EvalSymlinks(entrypoint)
	if err != nil {
		return err
	}
	if !pathWithin(containingRoot, canonicalEntrypoint) {
		return errors.New("Entrypoint index.php resolves outside the PHP Project")
	}
	return nil
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func looksLikeWindowsVolume(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':'
}
