package parser

import "testing"

func TestParseJSON_FlatKeys(t *testing.T) {
	cfg, err := ParseJSON([]byte(`{"host": "localhost", "port": 5432}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["host"].Raw != "localhost" {
		t.Errorf("host = %+v", cfg["host"])
	}
	if cfg["port"].Raw != "5432" || cfg["port"].Kind != "number" {
		t.Errorf("port = %+v, want number 5432", cfg["port"])
	}
}

func TestParseJSON_NestedFlattening(t *testing.T) {
	cfg, err := ParseJSON([]byte(`{"database": {"host": "localhost", "pool": {"size": 10}}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["database.host"].Raw != "localhost" {
		t.Errorf("database.host = %+v", cfg["database.host"])
	}
	if cfg["database.pool.size"].Raw != "10" {
		t.Errorf("database.pool.size = %+v", cfg["database.pool.size"])
	}
}

func TestParseJSON_ArrayFlattening(t *testing.T) {
	cfg, err := ParseJSON([]byte(`{"hosts": ["a", "b"]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["hosts.0"].Raw != "a" || cfg["hosts.1"].Raw != "b" {
		t.Errorf("unexpected array flattening: %+v", cfg)
	}
}

func TestParseJSON_BooleanAndNullKinds(t *testing.T) {
	cfg, err := ParseJSON([]byte(`{"enabled": true, "maybe": null}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["enabled"].Kind != "bool" {
		t.Errorf("enabled kind = %q, want bool", cfg["enabled"].Kind)
	}
	if cfg["maybe"].Kind != "null" {
		t.Errorf("maybe kind = %q, want null", cfg["maybe"].Kind)
	}
}

func TestParseJSON_InvalidJSONErrors(t *testing.T) {
	_, err := ParseJSON([]byte(`{not valid json`))
	if err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestParseJSON_WholeNumberFloatFormatting(t *testing.T) {
	cfg, err := ParseJSON([]byte(`{"port": 8080.0}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["port"].Raw != "8080" {
		t.Errorf("port = %q, want 8080 (no trailing .0)", cfg["port"].Raw)
	}
}
