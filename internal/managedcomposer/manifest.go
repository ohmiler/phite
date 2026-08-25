package managedcomposer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/ohmiler/phite/internal/portable"
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type artifact struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type notices struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type composerSpec struct {
	Version  string   `json:"version"`
	Artifact artifact `json:"artifact"`
	Notices  notices  `json:"notices"`
}

type manifest struct {
	Schema   int          `json:"schema"`
	Composer composerSpec `json:"composer"`
}

func parseManifest(data []byte) (composerSpec, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var catalog manifest
	if err := decoder.Decode(&catalog); err != nil {
		return composerSpec{}, fmt.Errorf("decode Composer Manifest: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if !errors.Is(err, io.EOF) {
		if err == nil {
			return composerSpec{}, errors.New("Composer Manifest contains trailing JSON values")
		}
		return composerSpec{}, fmt.Errorf("decode Composer Manifest: %w", err)
	}
	if catalog.Schema != 1 {
		return composerSpec{}, fmt.Errorf("unsupported Composer Manifest schema %d", catalog.Schema)
	}
	if err := validateComposer(catalog.Composer); err != nil {
		return composerSpec{}, err
	}
	return catalog.Composer, nil
}

func validateComposer(spec composerSpec) error {
	fields := map[string]string{
		"composer.version":         spec.Version,
		"composer.artifact.name":   spec.Artifact.Name,
		"composer.artifact.url":    spec.Artifact.URL,
		"composer.artifact.sha256": spec.Artifact.SHA256,
		"composer.notices.name":    spec.Notices.Name,
		"composer.notices.url":     spec.Notices.URL,
		"composer.notices.sha256":  spec.Notices.SHA256,
	}
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("Composer Manifest is missing %s", name)
		}
	}
	if !portable.Name(spec.Version) {
		return fmt.Errorf("Composer version %q must be a portable path component", spec.Version)
	}
	if err := validateLeafName("artifact.name", spec.Artifact.Name); err != nil {
		return err
	}
	if err := validateLeafName("notices.name", spec.Notices.Name); err != nil {
		return err
	}
	if !sha256Pattern.MatchString(spec.Artifact.SHA256) {
		return errors.New("Composer artifact has an invalid SHA-256")
	}
	if !sha256Pattern.MatchString(spec.Notices.SHA256) {
		return errors.New("Composer notices have an invalid SHA-256")
	}
	if err := validateArtifactURL(spec.Artifact.URL); err != nil {
		return fmt.Errorf("Composer artifact: %w", err)
	}
	if err := validateArtifactURL(spec.Notices.URL); err != nil {
		return fmt.Errorf("Composer notices: %w", err)
	}
	return nil
}

func validateLeafName(field, value string) error {
	if !portable.Name(value) {
		return fmt.Errorf("Composer Manifest %s %q is not a portable file name", field, value)
	}
	return nil
}

func validateArtifactURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	if parsed.Scheme == "https" && parsed.Host != "" {
		return nil
	}
	if parsed.Scheme == "http" && parsed.Host != "" {
		host := parsed.Hostname()
		if strings.EqualFold(host, "localhost") {
			return nil
		}
		if address := net.ParseIP(host); address != nil && address.IsLoopback() {
			return nil
		}
	}
	return errors.New("URL must use HTTPS, except for a loopback test server")
}
