package parser

import "testing"

func TestParseYAML_FlatKeys(t *testing.T) {
	cfg, err := ParseYAML([]byte("host: localhost\nport: 5432\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["host"].Raw != "localhost" {
		t.Errorf("host = %q, want localhost", cfg["host"].Raw)
	}
	if cfg["port"].Raw != "5432" || cfg["port"].Kind != "number" {
		t.Errorf("port = %+v, want number 5432", cfg["port"])
	}
}

func TestParseYAML_NestedFlattening(t *testing.T) {
	data := []byte(`
database:
  host: localhost
  credentials:
    user: admin
`)
	cfg, err := ParseYAML(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["database.host"].Raw != "localhost" {
		t.Errorf("database.host = %+v", cfg["database.host"])
	}
	if cfg["database.credentials.user"].Raw != "admin" {
		t.Errorf("database.credentials.user = %+v", cfg["database.credentials.user"])
	}
}

func TestParseYAML_ListFlattening(t *testing.T) {
	data := []byte("hosts:\n  - a.example.com\n  - b.example.com\n")
	cfg, err := ParseYAML(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["hosts.0"].Raw != "a.example.com" || cfg["hosts.1"].Raw != "b.example.com" {
		t.Errorf("unexpected list flattening: %+v", cfg)
	}
}

func TestParseYAML_BooleanKind(t *testing.T) {
	cfg, err := ParseYAML([]byte("feature_enabled: true\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["feature_enabled"].Kind != "bool" || cfg["feature_enabled"].Raw != "true" {
		t.Errorf("feature_enabled = %+v, want bool true", cfg["feature_enabled"])
	}
}

func TestParseYAML_QuotedStringStaysString(t *testing.T) {
	cfg, err := ParseYAML([]byte(`port: "5432"` + "\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["port"].Kind != "string" {
		t.Errorf("port kind = %q, want string (quoted in YAML)", cfg["port"].Kind)
	}
}

func TestParseYAML_NullValue(t *testing.T) {
	cfg, err := ParseYAML([]byte("optional:\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["optional"].Kind != "null" {
		t.Errorf("optional = %+v, want null", cfg["optional"])
	}
}

func TestParseYAML_InvalidYAMLErrors(t *testing.T) {
	_, err := ParseYAML([]byte("foo: [unterminated\n"))
	if err == nil {
		t.Fatal("expected an error for invalid YAML")
	}
}
