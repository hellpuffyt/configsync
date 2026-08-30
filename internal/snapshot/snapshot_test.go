package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prabeshsharma/configsync/internal/model"
	"github.com/prabeshsharma/configsync/internal/rules"
)

func cfg(pairs ...string) model.Config {
	c := make(model.Config)
	for i := 0; i+1 < len(pairs); i += 2 {
		c[pairs[i]] = model.Value{Raw: pairs[i+1], Kind: model.KindString}
	}
	return c
}

func TestBuild_CapturesAllKeysAndEnvironments(t *testing.T) {
	envs := []model.Environment{
		{Name: "dev", Config: cfg("A", "1", "B", "2")},
		{Name: "prod", Config: cfg("A", "1")},
	}
	snap := Build(envs, rules.Default())
	if len(snap.Environments) != 2 {
		t.Errorf("Environments = %v, want 2", snap.Environments)
	}
	if len(snap.Entries) != 2 {
		t.Fatalf("Entries = %d, want 2 (A and B)", len(snap.Entries))
	}
	if snap.Entries["A"]["dev"].Value != "1" {
		t.Errorf("A/dev = %+v, want value 1", snap.Entries["A"]["dev"])
	}
	if snap.Entries["B"]["prod"].Present {
		t.Error("B/prod should not be present")
	}
}

const secretVal = "top-secret-abc123"

func TestBuild_SecretsAreHashedNotStored(t *testing.T) {
	envs := []model.Environment{
		{Name: "dev", Config: cfg("API_KEY", secretVal)},
	}
	snap := Build(envs, rules.Default())
	entry := snap.Entries["API_KEY"]["dev"]
	if !entry.Secret {
		t.Fatal("expected API_KEY to be flagged secret")
	}
	if entry.Value != "" {
		t.Errorf("expected no plaintext value stored for a secret, got %q", entry.Value)
	}
	if entry.Hash == "" || !strings.HasPrefix(entry.Hash, "sha256:") {
		t.Errorf("expected a sha256 hash to be stored, got %q", entry.Hash)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	envs := []model.Environment{
		{Name: "dev", Config: cfg("A", "1")},
		{Name: "prod", Config: cfg("A", "2")},
	}
	snap := Build(envs, rules.Default())

	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	if err := Save(path, snap); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.Entries["A"]["dev"].Value != "1" {
		t.Errorf("loaded A/dev = %+v", loaded.Entries["A"]["dev"])
	}
}

func TestSaveLoad_SnapshotFileNeverContainsSecretPlaintext(t *testing.T) {
	envs := []model.Environment{
		{Name: "dev", Config: cfg("DB_PASSWORD", secretVal)},
	}
	snap := Build(envs, rules.Default())

	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	if err := Save(path, snap); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), secretVal) {
		t.Fatalf("secret value leaked into snapshot file:\n%s", data)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected error loading a missing snapshot")
	}
}

func TestDiff_DetectsValueChanged(t *testing.T) {
	old := Build([]model.Environment{
		{Name: "dev", Config: cfg("A", "1")},
	}, rules.Default())
	newer := Build([]model.Environment{
		{Name: "dev", Config: cfg("A", "2")},
	}, rules.Default())

	drifted := Diff(old, newer)
	if len(drifted) != 1 {
		t.Fatalf("drifted = %+v, want 1 entry", drifted)
	}
	if drifted[0].Key != "A" || drifted[0].Change != ValueChanged {
		t.Errorf("got %+v", drifted[0])
	}
}

func TestDiff_DetectsBecameMissing(t *testing.T) {
	old := Build([]model.Environment{
		{Name: "dev", Config: cfg("A", "1")},
	}, rules.Default())
	newer := Build([]model.Environment{
		{Name: "dev", Config: cfg()},
	}, rules.Default())

	drifted := Diff(old, newer)
	if len(drifted) != 1 || drifted[0].Change != BecameMissing {
		t.Fatalf("got %+v", drifted)
	}
}

func TestDiff_DetectsBecamePresent(t *testing.T) {
	old := Build([]model.Environment{
		{Name: "dev", Config: cfg()},
	}, rules.Default())
	newer := Build([]model.Environment{
		{Name: "dev", Config: cfg("NEW_KEY", "x")},
	}, rules.Default())

	drifted := Diff(old, newer)
	if len(drifted) != 1 || drifted[0].Change != BecamePresent {
		t.Fatalf("got %+v", drifted)
	}
}

func TestDiff_NoChangeReportsNothing(t *testing.T) {
	envs := []model.Environment{
		{Name: "dev", Config: cfg("A", "1")},
	}
	old := Build(envs, rules.Default())
	newer := Build(envs, rules.Default())

	drifted := Diff(old, newer)
	if len(drifted) != 0 {
		t.Fatalf("expected no drift, got %+v", drifted)
	}
}

func TestDiff_SecretValueChangeDetectedViaHashWithoutExposingValue(t *testing.T) {
	old := Build([]model.Environment{
		{Name: "dev", Config: cfg("API_KEY", "old-secret-value")},
	}, rules.Default())
	newer := Build([]model.Environment{
		{Name: "dev", Config: cfg("API_KEY", "new-secret-value")},
	}, rules.Default())

	drifted := Diff(old, newer)
	if len(drifted) != 1 {
		t.Fatalf("got %+v", drifted)
	}
	if !drifted[0].Secret {
		t.Error("expected drift entry to be marked secret")
	}
	if strings.Contains(drifted[0].Key+string(drifted[0].Change), "old-secret-value") {
		t.Error("drift entry should never contain the actual secret value")
	}
}

func TestDiff_SecretSameValueNoDrift(t *testing.T) {
	envs := []model.Environment{
		{Name: "dev", Config: cfg("API_KEY", "same-secret-value")},
	}
	old := Build(envs, rules.Default())
	newer := Build(envs, rules.Default())

	drifted := Diff(old, newer)
	if len(drifted) != 0 {
		t.Fatalf("expected no drift for an unchanged secret, got %+v", drifted)
	}
}

func TestDiff_MultipleEnvironments(t *testing.T) {
	old := Build([]model.Environment{
		{Name: "dev", Config: cfg("A", "1")},
		{Name: "prod", Config: cfg("A", "1")},
	}, rules.Default())
	newer := Build([]model.Environment{
		{Name: "dev", Config: cfg("A", "1")},
		{Name: "prod", Config: cfg("A", "2")},
	}, rules.Default())

	drifted := Diff(old, newer)
	if len(drifted) != 1 || drifted[0].Env != "prod" {
		t.Fatalf("got %+v", drifted)
	}
}
