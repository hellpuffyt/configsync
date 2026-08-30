// Package parser reads .env, YAML, JSON, TOML, and Java-style .properties
// configuration files and flattens them into model.Config so that formats
// with nested structure (YAML, JSON, TOML) can be compared key-for-key
// against flat formats (.env, .properties).
package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/prabeshsharma/configsync/internal/model"
)

// Format identifies a supported configuration file format.
type Format string

const (
	FormatEnv        Format = "env"
	FormatYAML       Format = "yaml"
	FormatJSON       Format = "json"
	FormatTOML       Format = "toml"
	FormatProperties Format = "properties"
)

// DetectFormat infers a file's format from its extension.
func DetectFormat(path string) (Format, error) {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))
	switch {
	case ext == ".yaml" || ext == ".yml":
		return FormatYAML, nil
	case ext == ".json":
		return FormatJSON, nil
	case ext == ".toml":
		return FormatTOML, nil
	case ext == ".properties":
		return FormatProperties, nil
	case ext == ".env" || strings.HasSuffix(base, ".env") || strings.Contains(base, "env"):
		return FormatEnv, nil
	default:
		return "", fmt.Errorf("cannot detect config format for %q (supported: .env, .yaml/.yml, .json, .toml, .properties)", path)
	}
}

// ParseFile reads and parses a configuration file, auto-detecting its
// format from the file extension.
func ParseFile(path string) (model.Config, Format, error) {
	format, err := DetectFormat(path)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("reading %q: %w", path, err)
	}
	cfg, err := ParseBytes(data, format)
	if err != nil {
		return nil, "", fmt.Errorf("parsing %q as %s: %w", path, format, err)
	}
	return cfg, format, nil
}

// ParseBytes parses raw file content using the given format.
func ParseBytes(data []byte, format Format) (model.Config, error) {
	switch format {
	case FormatEnv:
		return ParseEnv(data)
	case FormatYAML:
		return ParseYAML(data)
	case FormatJSON:
		return ParseJSON(data)
	case FormatTOML:
		return ParseTOML(data)
	case FormatProperties:
		return ParseProperties(data)
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}
