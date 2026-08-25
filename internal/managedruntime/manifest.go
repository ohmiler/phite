package managedruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	goruntime "runtime"
	"strings"
)

var (
	ErrUnsupportedPlatform = errors.New("no Managed Runtime is available for this platform")
	runtimeIDPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	sha256Pattern          = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Identity struct {
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
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type runtimeSpec struct {
	Identity Identity `json:"identity"`
	Artifact artifact `json:"artifact"`
	Notices  notices  `json:"notices"`
}

type manifest struct {
	Schema   int           `json:"schema"`
	Runtimes []runtimeSpec `json:"runtimes"`
}

func parseCurrentRuntime(data []byte) (runtimeSpec, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var catalog manifest
	if err := decoder.Decode(&catalog); err != nil {
		return runtimeSpec{}, fmt.Errorf("decode Runtime Manifest: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return runtimeSpec{}, err
	}
	if catalog.Schema != 1 {
		return runtimeSpec{}, fmt.Errorf("unsupported Runtime Manifest schema %d", catalog.Schema)
	}
	if len(catalog.Runtimes) == 0 {
		return runtimeSpec{}, errors.New("Runtime Manifest contains no runtimes")
	}

	currentOS := manifestOS()
	currentArch := manifestArch()
	var selected *runtimeSpec
	for index := range catalog.Runtimes {
		candidate := &catalog.Runtimes[index]
		if err := validateRuntime(*candidate); err != nil {
			return runtimeSpec{}, err
		}
		if candidate.Identity.OS != currentOS || candidate.Identity.Arch != currentArch {
			continue
		}
		if selected != nil {
			return runtimeSpec{}, fmt.Errorf("Runtime Manifest has duplicate entries for %s %s", currentOS, currentArch)
		}
		selected = candidate
	}
	if selected == nil {
		return runtimeSpec{}, fmt.Errorf("%w: %s %s", ErrUnsupportedPlatform, currentOS, currentArch)
	}
	return *selected, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("Runtime Manifest contains trailing JSON values")
	}
	return fmt.Errorf("decode Runtime Manifest: %w", err)
}

func validateRuntime(candidate runtimeSpec) error {
	identity := candidate.Identity
	if err := validateSupport(identity.Support); err != nil {
		return err
	}
	fields := map[string]string{
		"identity.id":                 identity.ID,
		"identity.frankenphp_version": identity.FrankenPHPVersion,
		"identity.php_version":        identity.PHPVersion,
		"identity.caddy_version":      identity.CaddyVersion,
		"identity.os":                 identity.OS,
		"identity.arch":               identity.Arch,
		"identity.support":            identity.Support,
		"artifact.name":               candidate.Artifact.Name,
		"artifact.url":                candidate.Artifact.URL,
		"artifact.sha256":             candidate.Artifact.SHA256,
		"notices.name":                candidate.Notices.Name,
		"notices.url":                 candidate.Notices.URL,
		"notices.sha256":              candidate.Notices.SHA256,
	}
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("Runtime Manifest is missing %s", name)
		}
	}
	if !validRuntimeID(identity.ID) {
		return fmt.Errorf("Runtime Identity ID %q must be a safe lowercase slug", identity.ID)
	}
	if err := validateLeafName("artifact.name", candidate.Artifact.Name); err != nil {
		return err
	}
	if err := validateLeafName("notices.name", candidate.Notices.Name); err != nil {
		return err
	}
	if len(identity.Extensions) == 0 {
		return fmt.Errorf("Runtime Identity %s declares no extensions", identity.ID)
	}
	seenExtensions := make(map[string]struct{}, len(identity.Extensions))
	for _, extension := range identity.Extensions {
		if strings.TrimSpace(extension) == "" {
			return fmt.Errorf("Runtime Identity %s declares an empty extension", identity.ID)
		}
		key := strings.ToLower(extension)
		if _, duplicate := seenExtensions[key]; duplicate {
			return fmt.Errorf("Runtime Identity %s declares duplicate extension %s", identity.ID, extension)
		}
		seenExtensions[key] = struct{}{}
	}
	if !sha256Pattern.MatchString(candidate.Artifact.SHA256) {
		return fmt.Errorf("Runtime artifact for %s has an invalid SHA-256", identity.ID)
	}
	if !sha256Pattern.MatchString(candidate.Notices.SHA256) {
		return fmt.Errorf("Runtime notices for %s have an invalid SHA-256", identity.ID)
	}
	if err := validateArtifactURL(candidate.Artifact.URL); err != nil {
		return fmt.Errorf("Runtime artifact for %s: %w", identity.ID, err)
	}
	if err := validateArtifactURL(candidate.Notices.URL); err != nil {
		return fmt.Errorf("Runtime notices for %s: %w", identity.ID, err)
	}
	return nil
}

func validateLeafName(field, value string) error {
	if !validRuntimeFileName(value) {
		return invalidRuntimeFileName(field, value)
	}
	if value == "." || value == ".." || strings.ContainsAny(value, `/\`) {
		return fmt.Errorf("Runtime Manifest %s %q must be a file name", field, value)
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

func manifestOS() string {
	if goruntime.GOOS == "darwin" {
		return "macos"
	}
	return goruntime.GOOS
}

func manifestArch() string {
	if goruntime.GOARCH == "amd64" {
		return "x64"
	}
	return goruntime.GOARCH
}
