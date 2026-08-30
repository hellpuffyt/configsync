package parser

import "testing"

func TestParseTOML_FlatKeys(t *testing.T) {
	cfg, err := ParseTOML([]byte("host = \"localhost\"\nport = 5432\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["host"].Raw != "localhost" || cfg["host"].Kind != "string" {
		t.Errorf("host = %+v", cfg["host"])
	}
	if cfg["port"].Raw != "5432" || cfg["port"].Kind != "number" {
		t.Errorf("port = %+v", cfg["port"])
	}
}

func TestParseTOML_TableHeaders(t *testing.T) {
	data := []byte("[database]\nhost = \"localhost\"\n\n[database.credentials]\nuser = \"admin\"\n")
	cfg, err := ParseTOML(data)
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

func TestParseTOML_DottedKeys(t *testing.T) {
	cfg, err := ParseTOML([]byte(`database.host = "localhost"` + "\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["database.host"].Raw != "localhost" {
		t.Errorf("database.host = %+v", cfg["database.host"])
	}
}

func TestParseTOML_BooleanAndFloat(t *testing.T) {
	cfg, err := ParseTOML([]byte("enabled = true\nratio = 0.5\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["enabled"].Kind != "bool" {
		t.Errorf("enabled kind = %q, want bool", cfg["enabled"].Kind)
	}
	if cfg["ratio"].Raw != "0.5" {
		t.Errorf("ratio = %q, want 0.5", cfg["ratio"].Raw)
	}
}

func TestParseTOML_InlineArray(t *testing.T) {
	cfg, err := ParseTOML([]byte(`hosts = ["a", "b", "c"]` + "\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["hosts.0"].Raw != "a" || cfg["hosts.2"].Raw != "c" {
		t.Errorf("unexpected array flattening: %+v", cfg)
	}
}

func TestParseTOML_CommentsIgnored(t *testing.T) {
	cfg, err := ParseTOML([]byte("# top comment\nhost = \"localhost\" # inline\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["host"].Raw != "localhost" {
		t.Errorf("host = %q, want localhost", cfg["host"].Raw)
	}
}

func TestParseTOML_EscapedStringValue(t *testing.T) {
	cfg, err := ParseTOML([]byte(`msg = "line1\nline2"` + "\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["msg"].Raw != "line1\nline2" {
		t.Errorf("msg = %q", cfg["msg"].Raw)
	}
}

func TestParseTOML_ArrayOfTables(t *testing.T) {
	data := []byte("[[servers]]\nname = \"a\"\n\n[[servers]]\nname = \"b\"\n")
	cfg, err := ParseTOML(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["servers.0.name"].Raw != "a" || cfg["servers.1.name"].Raw != "b" {
		t.Errorf("unexpected array-of-tables flattening: %+v", cfg)
	}
}

func TestParseTOML_MalformedLineErrors(t *testing.T) {
	_, err := ParseTOML([]byte("this is not toml\n"))
	if err == nil {
		t.Fatal("expected an error for malformed TOML")
	}
}
