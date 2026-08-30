package parser

import "testing"

func TestParseProperties_Basic(t *testing.T) {
	cfg, err := ParseProperties([]byte("db.host=localhost\ndb.port=5432\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["db.host"].Raw != "localhost" {
		t.Errorf("db.host = %q, want localhost", cfg["db.host"].Raw)
	}
}

func TestParseProperties_ColonSeparator(t *testing.T) {
	cfg, err := ParseProperties([]byte("db.host: localhost\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["db.host"].Raw != "localhost" {
		t.Errorf("db.host = %q, want localhost", cfg["db.host"].Raw)
	}
}

func TestParseProperties_WhitespaceSeparator(t *testing.T) {
	cfg, err := ParseProperties([]byte("db.host   localhost\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["db.host"].Raw != "localhost" {
		t.Errorf("db.host = %q, want localhost", cfg["db.host"].Raw)
	}
}

func TestParseProperties_CommentsBothStyles(t *testing.T) {
	cfg, err := ParseProperties([]byte("# comment\n! also a comment\nkey=value\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg) != 1 {
		t.Fatalf("len(cfg) = %d, want 1", len(cfg))
	}
}

func TestParseProperties_LineContinuation(t *testing.T) {
	cfg, err := ParseProperties([]byte("greeting=hello \\\n  world\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["greeting"].Raw != "hello world" {
		t.Errorf("greeting = %q, want %q", cfg["greeting"].Raw, "hello world")
	}
}

func TestParseProperties_EscapedColonInKey(t *testing.T) {
	cfg, err := ParseProperties([]byte(`key\:withcolon=value` + "\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["key:withcolon"].Raw != "value" {
		t.Errorf("got %+v", cfg)
	}
}

func TestParseProperties_EmptyValue(t *testing.T) {
	cfg, err := ParseProperties([]byte("empty=\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg["empty"].IsEmpty() {
		t.Errorf("expected empty value, got %+v", cfg["empty"])
	}
}
