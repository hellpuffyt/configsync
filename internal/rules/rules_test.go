package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatch_ExactAndWildcards(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"DB_URL", "DB_URL", true},
		{"DB_URL", "db_url", true}, // case-insensitive
		{"*_URL", "DATABASE_URL", true},
		{"*_URL", "URL_PREFIX", false},
		{"*_HOST", "REDIS_HOST", true},
		{"API_*", "API_KEY", true},
		{"API_*", "MY_API_KEY", false},
		{"*KEY*", "MY_API_KEY", true},
		{"*", "anything", true},
		{"FEATURE_*", "FEATURE_FLAG_X", true},
		{"TIMEOUT", "TIMEOUT_MS", false},
	}
	for _, c := range cases {
		got := Match(c.pattern, c.s)
		if got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.s, got, c.want)
		}
	}
}

func TestRules_DefaultSecretPatterns(t *testing.T) {
	r := Default()
	secretKeys := []string{"DB_PASSWORD", "API_KEY", "AUTH_TOKEN", "STRIPE_SECRET", "PRIVATE_CERT", "ADMIN_PWD"}
	for _, k := range secretKeys {
		if !r.IsSecret(k) {
			t.Errorf("IsSecret(%q) = false, want true (default patterns)", k)
		}
	}
	nonSecret := []string{"DB_HOST", "APP_NAME", "TIMEOUT_MS"}
	for _, k := range nonSecret {
		if r.IsSecret(k) {
			t.Errorf("IsSecret(%q) = true, want false", k)
		}
	}
}

func TestRules_CustomSecretPatterns(t *testing.T) {
	r := &Rules{SecretPatterns: []string{"INTERNAL_*"}}
	if !r.IsSecret("INTERNAL_ID") {
		t.Error("expected INTERNAL_ID to be flagged secret via custom pattern")
	}
}

func TestRules_ExpectedVariance(t *testing.T) {
	r := &Rules{ExpectedVariance: []string{"*_URL", "*_HOST"}}
	if !r.IsExpectedVariance("DATABASE_URL") {
		t.Error("expected DATABASE_URL to match expected-variance rule")
	}
	if r.IsExpectedVariance("FEATURE_FLAG") {
		t.Error("did not expect FEATURE_FLAG to match expected-variance rule")
	}
}

func TestRules_MustMatch(t *testing.T) {
	r := &Rules{MustMatch: []string{"FEATURE_*", "*_TIMEOUT"}}
	if !r.IsMustMatch("FEATURE_NEW_UI") {
		t.Error("expected FEATURE_NEW_UI to match must-match rule")
	}
	if !r.IsMustMatch("REQUEST_TIMEOUT") {
		t.Error("expected REQUEST_TIMEOUT to match must-match rule")
	}
}

func TestRules_Ignore(t *testing.T) {
	r := &Rules{Ignore: []string{"COMMENT*", "_INTERNAL_*"}}
	if !r.IsIgnored("COMMENT_FIELD") {
		t.Error("expected COMMENT_FIELD to be ignored")
	}
	if r.IsIgnored("DB_HOST") {
		t.Error("did not expect DB_HOST to be ignored")
	}
}

func TestRules_NestedKeyMatchesLeafSegment(t *testing.T) {
	r := &Rules{ExpectedVariance: []string{"*_URL"}}
	if !r.IsExpectedVariance("services.database.connection_url") {
		t.Error("expected a dotted path to match against its final segment")
	}
}

func TestRules_Load(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml")
	content := "expected_variance:\n  - \"*_URL\"\nmust_match:\n  - \"FEATURE_*\"\nignore:\n  - \"COMMENT*\"\nsecret_patterns:\n  - \"*_SIGNING_KEY\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsExpectedVariance("API_URL") {
		t.Error("expected API_URL to match loaded expected-variance rule")
	}
	if !r.IsMustMatch("FEATURE_X") {
		t.Error("expected FEATURE_X to match loaded must-match rule")
	}
	if !r.IsIgnored("COMMENT_1") {
		t.Error("expected COMMENT_1 to match loaded ignore rule")
	}
	if !r.IsSecret("JWT_SIGNING_KEY") {
		t.Error("expected JWT_SIGNING_KEY to match loaded secret pattern")
	}
}

func TestRules_LoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected error loading a missing rules file")
	}
}

func TestRules_NilRulesNeverPanics(t *testing.T) {
	var r *Rules
	if r.IsSecret("ANYTHING") {
		t.Error("nil rules should not report secrets from an empty pattern list")
	}
	if r.IsIgnored("ANYTHING") || r.IsExpectedVariance("ANYTHING") || r.IsMustMatch("ANYTHING") {
		t.Error("nil rules should never match")
	}
}
