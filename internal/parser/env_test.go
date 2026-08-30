package parser

import "testing"

func TestParseEnv_Basic(t *testing.T) {
	cfg, err := ParseEnv([]byte("DB_HOST=localhost\nDB_PORT=5432\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["DB_HOST"].Raw != "localhost" {
		t.Errorf("DB_HOST = %q, want localhost", cfg["DB_HOST"].Raw)
	}
	if cfg["DB_PORT"].Raw != "5432" {
		t.Errorf("DB_PORT = %q, want 5432", cfg["DB_PORT"].Raw)
	}
}

func TestParseEnv_CommentsAndBlankLines(t *testing.T) {
	cfg, err := ParseEnv([]byte("# a comment\n\nFOO=bar\n   \n# another\nBAZ=qux\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg) != 2 {
		t.Fatalf("len(cfg) = %d, want 2", len(cfg))
	}
	if cfg["FOO"].Raw != "bar" || cfg["BAZ"].Raw != "qux" {
		t.Errorf("unexpected values: %+v", cfg)
	}
}

func TestParseEnv_ExportPrefix(t *testing.T) {
	cfg, err := ParseEnv([]byte("export FOO=bar\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["FOO"].Raw != "bar" {
		t.Errorf("FOO = %q, want bar", cfg["FOO"].Raw)
	}
}

func TestParseEnv_DoubleQuotedWithEscapes(t *testing.T) {
	cfg, err := ParseEnv([]byte(`MSG="hello\nworld"` + "\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "hello\nworld"
	if cfg["MSG"].Raw != want {
		t.Errorf("MSG = %q, want %q", cfg["MSG"].Raw, want)
	}
}

func TestParseEnv_SingleQuotedLiteral(t *testing.T) {
	cfg, err := ParseEnv([]byte(`RAW='no $expansion here'` + "\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "no $expansion here"
	if cfg["RAW"].Raw != want {
		t.Errorf("RAW = %q, want %q", cfg["RAW"].Raw, want)
	}
}

func TestParseEnv_InlineComment(t *testing.T) {
	cfg, err := ParseEnv([]byte("PORT=8080 # the port\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["PORT"].Raw != "8080" {
		t.Errorf("PORT = %q, want 8080", cfg["PORT"].Raw)
	}
}

func TestParseEnv_EmptyValue(t *testing.T) {
	cfg, err := ParseEnv([]byte("EMPTY=\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["EMPTY"].Raw != "" || !cfg["EMPTY"].IsEmpty() {
		t.Errorf("EMPTY = %+v, want empty string", cfg["EMPTY"])
	}
}

func TestParseEnv_MissingEqualsIsError(t *testing.T) {
	_, err := ParseEnv([]byte("NOT_VALID_LINE\n"))
	if err == nil {
		t.Fatal("expected an error for a line with no '='")
	}
}

func TestParseEnv_AllValuesAreStringKind(t *testing.T) {
	cfg, err := ParseEnv([]byte("A=1\nB=true\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for k, v := range cfg {
		if v.Kind != "string" {
			t.Errorf("%s kind = %q, want string (env values are always strings)", k, v.Kind)
		}
	}
}
