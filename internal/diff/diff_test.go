package diff

import (
	"testing"

	"github.com/prabeshsharma/configsync/internal/model"
	"github.com/prabeshsharma/configsync/internal/rules"
)

func strCfg(pairs ...string) model.Config {
	cfg := make(model.Config)
	for i := 0; i+1 < len(pairs); i += 2 {
		cfg[pairs[i]] = model.Value{Raw: pairs[i+1], Kind: model.KindString}
	}
	return cfg
}

func env(name string, cfg model.Config) model.Environment {
	return model.Environment{Name: name, Config: cfg}
}

func findEntry(m Matrix, key string) (Entry, bool) {
	for _, e := range m.Entries {
		if e.Key == key {
			return e, true
		}
	}
	return Entry{}, false
}

func TestCompare_IdenticalValuesAreOK(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("APP_NAME", "widget")),
		env("prod", strCfg("APP_NAME", "widget")),
	}
	m := Compare(envs, rules.Default())
	e, ok := findEntry(m, "APP_NAME")
	if !ok {
		t.Fatal("expected APP_NAME in matrix")
	}
	if e.Classification != OK || e.Unexpected {
		t.Errorf("got %+v, want OK and not unexpected", e)
	}
}

func TestCompare_MissingKey(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("FEATURE_X", "on")),
		env("prod", strCfg()),
	}
	m := Compare(envs, rules.Default())
	e, ok := findEntry(m, "FEATURE_X")
	if !ok {
		t.Fatal("expected FEATURE_X in matrix")
	}
	if e.Classification != Missing || !e.Unexpected {
		t.Errorf("got %+v, want Missing and unexpected", e)
	}
	if e.Cells["prod"].Present {
		t.Error("prod cell should not be present")
	}
}

func TestCompare_ValueDiffers(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("TIMEOUT_MS", "1000")),
		env("prod", strCfg("TIMEOUT_MS", "5000")),
	}
	m := Compare(envs, rules.Default())
	e, _ := findEntry(m, "TIMEOUT_MS")
	if e.Classification != ValueDiffers || !e.Unexpected {
		t.Errorf("got %+v, want ValueDiffers and unexpected", e)
	}
}

func TestCompare_EmptyIn(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("API_TOKEN_HINT", "")),
		env("prod", strCfg("API_TOKEN_HINT", "present")),
	}
	m := Compare(envs, rules.Default())
	e, _ := findEntry(m, "API_TOKEN_HINT")
	if e.Classification != EmptyIn || !e.Unexpected {
		t.Errorf("got %+v, want EmptyIn and unexpected", e)
	}
}

func TestCompare_EmptyEverywhereIsOK(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("NOTES", "")),
		env("prod", strCfg("NOTES", "")),
	}
	m := Compare(envs, rules.Default())
	e, _ := findEntry(m, "NOTES")
	if e.Classification != OK {
		t.Errorf("got %+v, want OK (empty in every environment is not drift)", e)
	}
}

func TestCompare_TypeDiffers(t *testing.T) {
	envs := []model.Environment{
		env("dev", model.Config{"DEBUG": {Raw: "true", Kind: model.KindString}}),
		env("prod", model.Config{"DEBUG": {Raw: "true", Kind: model.KindBool}}),
	}
	m := Compare(envs, rules.Default())
	e, _ := findEntry(m, "DEBUG")
	if e.Classification != TypeDiffers || !e.Unexpected {
		t.Errorf("got %+v, want TypeDiffers and unexpected", e)
	}
}

func TestCompare_ExpectedVarianceSuppressesValueDiffers(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("DATABASE_URL", "postgres://dev")),
		env("prod", strCfg("DATABASE_URL", "postgres://prod")),
	}
	r := &rules.Rules{ExpectedVariance: []string{"*_URL"}}
	m := Compare(envs, r)
	e, _ := findEntry(m, "DATABASE_URL")
	if e.Classification != ExpectedVariance || e.Unexpected {
		t.Errorf("got %+v, want ExpectedVariance and not unexpected", e)
	}
	if e.BaseClassification != ValueDiffers {
		t.Errorf("base classification = %v, want ValueDiffers", e.BaseClassification)
	}
}

func TestCompare_ExpectedVarianceSuppressesMissing(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("STAGING_ONLY_HOST", "x")),
		env("prod", strCfg()),
	}
	r := &rules.Rules{ExpectedVariance: []string{"*_HOST"}}
	m := Compare(envs, r)
	e, _ := findEntry(m, "STAGING_ONLY_HOST")
	if e.Classification != ExpectedVariance || e.Unexpected {
		t.Errorf("got %+v, want ExpectedVariance and not unexpected", e)
	}
}

func TestCompare_ExpectedVarianceDoesNotSuppressTypeDiffers(t *testing.T) {
	envs := []model.Environment{
		env("dev", model.Config{"SERVICE_URL": {Raw: "true", Kind: model.KindString}}),
		env("prod", model.Config{"SERVICE_URL": {Raw: "true", Kind: model.KindBool}}),
	}
	r := &rules.Rules{ExpectedVariance: []string{"*_URL"}}
	m := Compare(envs, r)
	e, _ := findEntry(m, "SERVICE_URL")
	if e.Classification != TypeDiffers || !e.Unexpected {
		t.Errorf("got %+v, want TypeDiffers to remain unexpected even with an expected-variance rule", e)
	}
}

func TestCompare_MustMatchEscalatesEvenIfExpectedVarianceAlsoMatches(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("FEATURE_URL_FLAG", "a")),
		env("prod", strCfg("FEATURE_URL_FLAG", "b")),
	}
	r := &rules.Rules{
		MustMatch:        []string{"FEATURE_*"},
		ExpectedVariance: []string{"*_URL_FLAG"},
	}
	m := Compare(envs, r)
	e, _ := findEntry(m, "FEATURE_URL_FLAG")
	if !e.MustMatchViolation || !e.Unexpected {
		t.Errorf("got %+v, want must-match violation to win over expected-variance", e)
	}
	if e.Classification != ValueDiffers {
		t.Errorf("classification = %v, want ValueDiffers (real class preserved on violation)", e.Classification)
	}
}

func TestCompare_MustMatchEscalatesMissing(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("FEATURE_X", "on")),
		env("prod", strCfg()),
	}
	r := &rules.Rules{MustMatch: []string{"FEATURE_*"}}
	m := Compare(envs, r)
	e, _ := findEntry(m, "FEATURE_X")
	if !e.MustMatchViolation || !e.Unexpected {
		t.Errorf("got %+v, want must-match violation on missing key", e)
	}
}

func TestCompare_IgnoreRuleExcludesKey(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("SCRATCH_NOTE", "a")),
		env("prod", strCfg("SCRATCH_NOTE", "b")),
	}
	r := &rules.Rules{Ignore: []string{"SCRATCH_*"}}
	m := Compare(envs, r)
	if _, ok := findEntry(m, "SCRATCH_NOTE"); ok {
		t.Error("expected SCRATCH_NOTE to be excluded by ignore rule")
	}
}

func TestCompare_ThreeWayComparison(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("REPLICAS", "1")),
		env("staging", strCfg("REPLICAS", "2")),
		env("prod", strCfg("REPLICAS", "3")),
	}
	m := Compare(envs, rules.Default())
	if len(m.Environments) != 3 {
		t.Fatalf("environments = %v, want 3", m.Environments)
	}
	e, _ := findEntry(m, "REPLICAS")
	if e.Classification != ValueDiffers {
		t.Errorf("got %+v, want ValueDiffers across three environments", e)
	}
	if len(e.Cells) != 3 {
		t.Errorf("cells = %d, want 3", len(e.Cells))
	}
}

func TestCompare_ThreeWayMissingInMiddleEnv(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("X", "1")),
		env("staging", strCfg()),
		env("prod", strCfg("X", "1")),
	}
	m := Compare(envs, rules.Default())
	e, _ := findEntry(m, "X")
	if e.Classification != Missing {
		t.Errorf("got %+v, want Missing", e)
	}
	if e.Cells["staging"].Present {
		t.Error("staging should be absent")
	}
}

func TestCompare_SecretFlagIsSetOnEntry(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("DB_PASSWORD", "hunter2")),
		env("prod", strCfg("DB_PASSWORD", "correcthorse")),
	}
	m := Compare(envs, rules.Default())
	e, _ := findEntry(m, "DB_PASSWORD")
	if !e.Secret {
		t.Error("expected DB_PASSWORD to be flagged as secret")
	}
	if e.Classification != ValueDiffers {
		t.Errorf("classification = %v, want ValueDiffers (secret status does not change classification)", e.Classification)
	}
}

func TestCompare_UnexpectedCount(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("A", "1", "B", "1")),
		env("prod", strCfg("A", "1", "B", "2")),
	}
	m := Compare(envs, rules.Default())
	if got := m.UnexpectedCount(); got != 1 {
		t.Errorf("UnexpectedCount() = %d, want 1", got)
	}
}

func TestCompare_NilRulesUsesDefaults(t *testing.T) {
	envs := []model.Environment{
		env("dev", strCfg("DB_PASSWORD", "a")),
		env("prod", strCfg("DB_PASSWORD", "b")),
	}
	m := Compare(envs, nil)
	e, _ := findEntry(m, "DB_PASSWORD")
	if !e.Secret {
		t.Error("expected default secret patterns to apply when rules is nil")
	}
}
