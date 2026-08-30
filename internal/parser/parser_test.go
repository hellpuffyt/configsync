package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFormat(t *testing.T) {
	cases := map[string]Format{
		"config.env":        FormatEnv,
		".env":              FormatEnv,
		"config.yaml":       FormatYAML,
		"config.yml":        FormatYAML,
		"config.json":       FormatJSON,
		"config.toml":       FormatTOML,
		"config.properties": FormatProperties,
	}
	for name, want := range cases {
		got, err := DetectFormat(name)
		if err != nil {
			t.Errorf("DetectFormat(%q) error: %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("DetectFormat(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestDetectFormat_Unknown(t *testing.T) {
	_, err := DetectFormat("config.xyz")
	if err == nil {
		t.Fatal("expected error for unrecognized extension")
	}
}

func TestParseFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.json")
	if err := os.WriteFile(path, []byte(`{"a": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, format, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatJSON {
		t.Errorf("format = %q, want json", format)
	}
	if cfg["a"].Raw != "1" {
		t.Errorf("a = %+v", cfg["a"])
	}
}

func TestParseFile_MissingFile(t *testing.T) {
	_, _, err := ParseFile(filepath.Join(t.TempDir(), "does-not-exist.env"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
