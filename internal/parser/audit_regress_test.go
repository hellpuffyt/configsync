package parser

import (
	"strings"
	"testing"

	"github.com/hellpuffyt/configsync/internal/model"
)

// Regression tests for parser defects found in the production-readiness
// audit. Each case silently produced a wrong value before.

func TestEnvDoubleQuotedEscapes(t *testing.T) {
	cfg, err := ParseEnv([]byte(`A="a\\nb"` + "\n" + `B="x\\\"y"` + "\n" + `C="tab\there"` + "\n" + `D="back\\slash"` + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"A": `a\nb`, // escaped backslash then a literal n, not a newline
		"B": `x\"y`,
		"C": "tab\there",
		"D": `back\slash`,
	}
	for k, w := range want {
		if got := cfg[k].Raw; got != w {
			t.Errorf("%s = %q, want %q", k, got, w)
		}
	}
}

func TestPropertiesUnicodeEscapes(t *testing.T) {
	cfg, err := ParseProperties([]byte("greeting=caf\\u00e9\nk\\u0041y=v\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg["greeting"].Raw != "café" {
		t.Errorf("greeting = %q, want café", cfg["greeting"].Raw)
	}
	if _, ok := cfg["kAy"]; !ok {
		t.Errorf("unicode escape in key not decoded: %v", cfg)
	}
}

// JSON integers above 2^53 lost precision, so two different IDs compared
// equal; whole-number floats still match YAML integers.
func TestJSONKeepsLargeIntegersExact(t *testing.T) {
	a, err := ParseJSON([]byte(`{"id": 9007199254740993, "port": 8080, "ratio": 1.0, "big": 1e300}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseJSON([]byte(`{"id": 9007199254740992}`))
	if err != nil {
		t.Fatal(err)
	}
	if a["id"].Raw == b["id"].Raw {
		t.Fatalf("distinct large integers compare equal: %q", a["id"].Raw)
	}
	if a["id"].Raw != "9007199254740993" || a["id"].Kind != model.KindNumber {
		t.Errorf("id = %+v", a["id"])
	}
	y, err := ParseYAML([]byte("port: 8080\nratio: 1\nbig: 18446744073709551615\n"))
	if err != nil {
		t.Fatal(err)
	}
	if a["port"].Raw != y["port"].Raw || a["ratio"].Raw != y["ratio"].Raw {
		t.Errorf("JSON/YAML numbers should normalise alike: %+v vs %+v", a, y)
	}
	if y["big"].Raw != "18446744073709551615" || y["big"].Kind != model.KindNumber {
		t.Errorf("uint64 overflowed: %+v", y["big"])
	}
	if a["big"].Raw != "1e+300" {
		t.Errorf("huge float rendered as %q", a["big"].Raw)
	}
}

func TestTOMLRejectsWhatItCannotParse(t *testing.T) {
	cases := map[string]string{
		"inline table":        `db = { host = "x", port = 5 }`,
		"multi-line basic":    "s = \"\"\"\nline\n\"\"\"",
		"multi-line literal":  "s = '''\nline\n'''",
		"bare word":           `mode = production`,
		"duplicate key":       "a = 1\na = 2",
		"key redefines table": "a.b = 1\na = 2",
	}
	for name, src := range cases {
		if _, err := ParseTOML([]byte(src)); err == nil {
			t.Errorf("%s: accepted silently", name)
		}
	}
}

func TestTOMLValuesItSupports(t *testing.T) {
	cfg, err := ParseTOML([]byte(strings.Join([]string{
		`hex = 0x1F`,
		`oct = 0o17`,
		`bin = 0b101`,
		`neg = -12_000`,
		`when = 1979-05-27T07:32:00Z`,
		`day = 1979-05-27`,
		`esc = "a \" # not a comment"   # real comment`,
		`arr = ["a,b", "c]d"]`,
		`lit = 'C:\path'`,
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"hex": "31", "oct": "15", "bin": "5", "neg": "-12000",
		"when": "1979-05-27T07:32:00Z", "day": "1979-05-27",
		"esc": `a " # not a comment`, "arr.0": "a,b", "arr.1": "c]d", "lit": `C:\path`,
	}
	for k, w := range want {
		if got := cfg[k].Raw; got != w {
			t.Errorf("%s = %q, want %q", k, got, w)
		}
	}
	if cfg["hex"].Kind != model.KindNumber {
		t.Errorf("hex kind = %s", cfg["hex"].Kind)
	}
}
