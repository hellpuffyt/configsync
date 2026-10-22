package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(t *testing.T, args ...string) (string, int, error) {
	t.Helper()
	var out bytes.Buffer
	code, err := run(args, &out)
	return out.String(), code, err
}

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// The committed examples produce the README's table and exit 1.
func TestCompareExamples(t *testing.T) {
	out, code, err := runCLI(t, "compare",
		"--env", "dev=../../examples/dev.env",
		"--env", "staging=../../examples/staging.yaml",
		"--env", "prod=../../examples/prod.json",
		"--rules", "../../examples/rules.yaml")
	if err != nil || code != 1 {
		t.Fatalf("exit %d err %v", code, err)
	}
	for _, want := range []string{"FEATURE_NEW_UI", "type-differs (must-match)", "STAGING_ONLY_FLAG", "7 key(s) shown, 5 unexpected drift"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	for _, secret := range []string{"dev-local-placeholder-pw", "staging-secret-pw", "prod-secret-pw", "fake-key"} {
		if strings.Contains(out, secret) {
			t.Fatalf("secret %q leaked:\n%s", secret, out)
		}
	}
}

// Regression: a password inside a URL value (DATABASE_URL is not a secret
// key name) was printed in table and JSON output and stored in snapshots.
func TestURLPasswordsNeverPrintedOrStored(t *testing.T) {
	dir := t.TempDir()
	pw1, pw2 := "hunter"+"2-dev", "hunter"+"2-prod"
	a := write(t, dir, "a.env", "DATABASE_URL=postgres://app:"+pw1+"@db-dev/app\n")
	b := write(t, dir, "b.env", "DATABASE_URL=postgres://app:"+pw2+"@db-prod/app\n")
	for _, format := range []string{"table", "json"} {
		out, code, err := runCLI(t, "compare", "--env", "dev="+a, "--env", "prod="+b, "--format", format)
		if err != nil || code != 1 {
			t.Fatalf("%s: exit %d err %v", format, code, err)
		}
		if strings.Contains(out, "hunter") {
			t.Fatalf("%s output leaked a URL password:\n%s", format, out)
		}
		if !strings.Contains(out, "postgres://app:xxxxx@db-dev/app") {
			t.Fatalf("%s output should show the redacted URL:\n%s", format, out)
		}
	}
	snap := filepath.Join(dir, "snap.json")
	if _, _, err := runCLI(t, "snapshot", "--env", "dev="+a, "--env", "prod="+b, "--out", snap); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(snap)
	if strings.Contains(string(data), "hunter") {
		t.Fatalf("snapshot stored a URL password:\n%s", data)
	}
	// A password-only change is still detected as drift.
	write(t, dir, "b.env", "DATABASE_URL=postgres://app:rotated@db-prod/app\n")
	out, code, err := runCLI(t, "drift", "--snapshot", snap, "--env", "dev="+a, "--env", "prod="+b, "--format", "json")
	if err != nil || code != 1 || !strings.Contains(out, `"value-changed"`) {
		t.Fatalf("password change not detected: exit %d err %v\n%s", code, err, out)
	}
}

// Regression: `ignore` keys were recorded in snapshots and reported by
// drift, so a changing GENERATED_AT failed every drift run.
func TestDriftHonoursIgnore(t *testing.T) {
	dir := t.TempDir()
	rules := write(t, dir, "rules.yaml", "ignore:\n  - GENERATED_AT\n")
	a := write(t, dir, "a.env", "A=1\nGENERATED_AT=100\n")
	b := write(t, dir, "b.env", "A=1\nGENERATED_AT=100\n")
	snap := filepath.Join(dir, "snap.json")
	if _, _, err := runCLI(t, "snapshot", "--env", "a="+a, "--env", "b="+b, "--rules", rules, "--out", snap); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(snap)
	if strings.Contains(string(data), "GENERATED_AT") {
		t.Fatalf("ignored key stored in snapshot:\n%s", data)
	}
	write(t, dir, "a.env", "A=1\nGENERATED_AT=200\n")
	out, code, err := runCLI(t, "drift", "--snapshot", snap, "--env", "a="+a, "--env", "b="+b, "--rules", rules)
	if err != nil || code != 0 {
		t.Fatalf("ignored key reported as drift: exit %d err %v\n%s", code, err, out)
	}
	// A snapshot taken without the rule still does not report it now.
	snap2 := filepath.Join(dir, "snap2.json")
	runCLI(t, "snapshot", "--env", "a="+a, "--env", "b="+b, "--out", snap2)
	write(t, dir, "a.env", "A=1\nGENERATED_AT=300\n")
	if _, code, _ := runCLI(t, "drift", "--snapshot", snap2, "--env", "a="+a, "--env", "b="+b, "--rules", rules); code != 0 {
		t.Fatalf("ignored key from an older snapshot reported: exit %d", code)
	}
}

// Regression: a repeated environment name silently replaced the first one.
func TestDuplicateEnvironmentNamesRejected(t *testing.T) {
	dir := t.TempDir()
	a := write(t, dir, "a.env", "A=1\n")
	b := write(t, dir, "b.env", "A=2\n")
	_, _, err := runCLI(t, "compare", "--env", "prod="+a, "--env", "prod="+b)
	if err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("want duplicate-name error, got %v", err)
	}
}

func TestUsageErrorsAndHelp(t *testing.T) {
	if _, _, err := runCLI(t, "compare", "--env", "a=x.env", "--env", "b=y.env", "--format", "xml"); err == nil {
		t.Error("bad --format accepted")
	}
	if _, _, err := runCLI(t, "compare", "--env", "a=x.env"); err == nil {
		t.Error("single --env accepted")
	}
	if _, _, err := runCLI(t, "compare", "-h"); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("-h: %v", err)
	}
	out, code, err := runCLI(t, "version")
	if err != nil || code != 0 || !strings.Contains(out, "configsync 0.1.0") {
		t.Errorf("version: %q %d %v", out, code, err)
	}
}

func TestCompareJSONIsValid(t *testing.T) {
	out, code, err := runCLI(t, "compare",
		"--env", "dev=../../examples/dev.env",
		"--env", "prod=../../examples/prod.json",
		"--format", "json", "--exit-zero")
	if err != nil || code != 0 {
		t.Fatalf("exit %d err %v", code, err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}
