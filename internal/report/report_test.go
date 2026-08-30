package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/prabeshsharma/configsync/internal/diff"
	"github.com/prabeshsharma/configsync/internal/model"
	"github.com/prabeshsharma/configsync/internal/rules"
)

const secretValue = "s3cr3t-super-sensitive-value-xyz123"

func buildSecretMatrix() diff.Matrix {
	envs := []model.Environment{
		{Name: "dev", Config: model.Config{
			"DB_PASSWORD": {Raw: secretValue, Kind: model.KindString},
			"APP_NAME":    {Raw: "widget", Kind: model.KindString},
		}},
		{Name: "prod", Config: model.Config{
			"DB_PASSWORD": {Raw: secretValue + "-prod", Kind: model.KindString},
			"APP_NAME":    {Raw: "widget", Kind: model.KindString},
		}},
	}
	return diff.Compare(envs, rules.Default())
}

// TestTable_SecretValueNeverAppears is the most important test in the
// suite: it asserts that a secret value is never written to the human
// table output, in any form.
func TestTable_SecretValueNeverAppears(t *testing.T) {
	m := buildSecretMatrix()
	var buf bytes.Buffer
	Table(&buf, m, false)
	out := buf.String()
	if strings.Contains(out, secretValue) {
		t.Fatalf("secret value leaked into table output:\n%s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("expected [REDACTED] marker in table output:\n%s", out)
	}
}

// TestJSON_SecretValueNeverAppears mirrors the table test for JSON output.
func TestJSON_SecretValueNeverAppears(t *testing.T) {
	m := buildSecretMatrix()
	var buf bytes.Buffer
	if err := JSON(&buf, m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, secretValue) {
		t.Fatalf("secret value leaked into JSON output:\n%s", out)
	}
}

func TestJSON_SecretEntryMarkedRedacted(t *testing.T) {
	m := buildSecretMatrix()
	doc := BuildDocument(m)
	for _, e := range doc.Entries {
		if e.Key != "DB_PASSWORD" {
			continue
		}
		if !e.Secret {
			t.Error("expected DB_PASSWORD entry to be marked secret")
		}
		for env, cell := range e.Values {
			if !cell.Redacted {
				t.Errorf("expected %s cell for DB_PASSWORD to be marked redacted", env)
			}
			if cell.Value != "" {
				t.Errorf("expected empty value for redacted cell, got %q", cell.Value)
			}
		}
	}
}

func TestJSON_NonSecretValuesStillAppear(t *testing.T) {
	m := buildSecretMatrix()
	var buf bytes.Buffer
	if err := JSON(&buf, m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "widget") {
		t.Error("expected non-secret value 'widget' to appear in JSON output")
	}
}

func TestTable_MissingAndEmptyMarkers(t *testing.T) {
	envs := []model.Environment{
		{Name: "dev", Config: model.Config{"KEY": {Raw: "", Kind: model.KindString}}},
		{Name: "prod", Config: model.Config{"KEY": {Raw: "value", Kind: model.KindString}}},
	}
	m := diff.Compare(envs, rules.Default())
	var buf bytes.Buffer
	Table(&buf, m, false)
	out := buf.String()
	if !strings.Contains(out, "<empty>") {
		t.Errorf("expected <empty> marker in output:\n%s", out)
	}

	envs2 := []model.Environment{
		{Name: "dev", Config: model.Config{"ONLY_DEV": {Raw: "x", Kind: model.KindString}}},
		{Name: "prod", Config: model.Config{}},
	}
	m2 := diff.Compare(envs2, rules.Default())
	var buf2 bytes.Buffer
	Table(&buf2, m2, false)
	if !strings.Contains(buf2.String(), "<missing>") {
		t.Errorf("expected <missing> marker in output:\n%s", buf2.String())
	}
}

func TestTable_DiffOnlyFiltersOKEntries(t *testing.T) {
	envs := []model.Environment{
		{Name: "dev", Config: model.Config{"SAME": {Raw: "x", Kind: model.KindString}, "DIFF": {Raw: "a", Kind: model.KindString}}},
		{Name: "prod", Config: model.Config{"SAME": {Raw: "x", Kind: model.KindString}, "DIFF": {Raw: "b", Kind: model.KindString}}},
	}
	m := diff.Compare(envs, rules.Default())

	var full bytes.Buffer
	Table(&full, m, false)
	if !strings.Contains(full.String(), "SAME") {
		t.Error("expected SAME to appear when diffOnly is false")
	}

	var filtered bytes.Buffer
	Table(&filtered, m, true)
	if strings.Contains(filtered.String(), "SAME") {
		t.Error("did not expect SAME (an OK entry) to appear when diffOnly is true")
	}
	if !strings.Contains(filtered.String(), "DIFF") {
		t.Error("expected DIFF to still appear when diffOnly is true")
	}
}

func TestJSON_SummaryCounts(t *testing.T) {
	m := buildSecretMatrix()
	doc := BuildDocument(m)
	if doc.Summary.TotalKeys != 2 {
		t.Errorf("TotalKeys = %d, want 2", doc.Summary.TotalKeys)
	}
	if doc.Summary.Unexpected != 1 {
		t.Errorf("Unexpected = %d, want 1 (DB_PASSWORD differs)", doc.Summary.Unexpected)
	}
	if doc.Summary.OK != 1 {
		t.Errorf("OK = %d, want 1 (APP_NAME matches)", doc.Summary.OK)
	}
}

func TestJSON_ValidJSONOutput(t *testing.T) {
	m := buildSecretMatrix()
	var buf bytes.Buffer
	if err := JSON(&buf, m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, ok := decoded["entries"]; !ok {
		t.Error("expected 'entries' key in JSON output")
	}
}

func TestTable_EnvironmentColumnsPresent(t *testing.T) {
	m := buildSecretMatrix()
	var buf bytes.Buffer
	Table(&buf, m, false)
	out := buf.String()
	if !strings.Contains(out, "dev") || !strings.Contains(out, "prod") {
		t.Errorf("expected environment names as column headers:\n%s", out)
	}
}
