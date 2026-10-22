// Command configsync detects configuration drift between two or more
// environments (dev/staging/prod), classifying each difference so the
// important distinctions -- a key that differs, a key that is missing, and
// a key that is merely empty -- are never conflated the way a plain diff
// conflates them.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/hellpuffyt/configsync/internal/diff"
	"github.com/hellpuffyt/configsync/internal/model"
	"github.com/hellpuffyt/configsync/internal/parser"
	"github.com/hellpuffyt/configsync/internal/report"
	"github.com/hellpuffyt/configsync/internal/rules"
	"github.com/hellpuffyt/configsync/internal/snapshot"
)

const version = "0.1.0"

func main() {
	code, err := run(os.Args[1:], os.Stdout)
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0) // -h on a subcommand: usage was printed
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "configsync:", err)
		os.Exit(2)
	}
	os.Exit(code)
}

// run executes one command and returns its exit code: 0 clean, 1 drift
// found. An error means a usage or input problem (exit 2).
func run(args []string, stdout io.Writer) (int, error) {
	if len(args) == 0 {
		printUsage()
		return 0, fmt.Errorf("missing command")
	}

	switch args[0] {
	case "compare":
		return runCompare(args[1:], stdout)
	case "snapshot":
		return 0, runSnapshot(args[1:], stdout)
	case "drift":
		return runDrift(args[1:], stdout)
	case "version":
		fmt.Fprintln(stdout, "configsync", version)
		return 0, nil
	case "-h", "--help", "help":
		printUsage()
		return 0, nil
	default:
		printUsage()
		return 0, fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `configsync -- detect configuration drift between environments

Usage:
  configsync compare  --env NAME=PATH [--env NAME=PATH ...] [--rules FILE] [--format table|json] [--all] [--exit-zero]
  configsync snapshot --env NAME=PATH [--env NAME=PATH ...] [--rules FILE] --out FILE
  configsync drift    --snapshot FILE --env NAME=PATH [--env NAME=PATH ...] [--rules FILE] [--format table|json] [--exit-zero]
  configsync version

Supported formats (auto-detected by extension): .env, .yaml/.yml, .json, .toml, .properties`)
}

// envFlags implements flag.Value to collect repeated --env NAME=PATH flags.
type envFlags struct {
	entries []string
}

func (e *envFlags) String() string { return strings.Join(e.entries, ",") }
func (e *envFlags) Set(v string) error {
	e.entries = append(e.entries, v)
	return nil
}

func (e *envFlags) parse() ([]model.Environment, error) {
	if len(e.entries) < 2 {
		return nil, fmt.Errorf("at least 2 --env flags are required (got %d)", len(e.entries))
	}
	var envs []model.Environment
	seen := map[string]bool{}
	for _, ent := range e.entries {
		name, path, ok := strings.Cut(ent, "=")
		if !ok || name == "" || path == "" {
			return nil, fmt.Errorf("invalid --env value %q, expected NAME=PATH", ent)
		}
		if seen[name] {
			// Cells are keyed by environment name; a repeated name would
			// silently overwrite one environment with another.
			return nil, fmt.Errorf("environment name %q given more than once", name)
		}
		seen[name] = true
		cfg, format, err := parser.ParseFile(path)
		if err != nil {
			return nil, err
		}
		envs = append(envs, model.Environment{Name: name, Path: path, Format: string(format), Config: cfg})
	}
	return envs, nil
}

func loadRules(path string) (*rules.Rules, error) {
	if path == "" {
		return rules.Default(), nil
	}
	return rules.Load(path)
}

func runCompare(args []string, stdout io.Writer) (int, error) {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	var envs envFlags
	fs.Var(&envs, "env", "NAME=PATH, repeatable, at least 2 required")
	rulesPath := fs.String("rules", "", "path to a rules YAML file")
	format := fs.String("format", "table", "output format: table|json")
	all := fs.Bool("all", false, "show all keys, including keys that match everywhere (table format only)")
	exitZero := fs.Bool("exit-zero", false, "always exit 0, even if unexpected drift is found")
	if err := fs.Parse(args); err != nil {
		return 0, err
	}
	if *format != "table" && *format != "json" {
		return 0, fmt.Errorf("unknown --format %q (want table or json)", *format)
	}

	parsedEnvs, err := envs.parse()
	if err != nil {
		return 0, err
	}
	r, err := loadRules(*rulesPath)
	if err != nil {
		return 0, err
	}

	matrix := diff.Compare(parsedEnvs, r)
	if err := writeReport(stdout, matrix, *format, !*all); err != nil {
		return 0, err
	}

	if !*exitZero && matrix.UnexpectedCount() > 0 {
		return 1, nil
	}
	return 0, nil
}

func writeReport(w io.Writer, matrix diff.Matrix, format string, diffOnly bool) error {
	switch format {
	case "table":
		report.Table(w, matrix, diffOnly)
		return nil
	case "json":
		return report.JSON(w, matrix)
	default:
		return fmt.Errorf("unknown --format %q (want table or json)", format)
	}
}

func runSnapshot(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	var envs envFlags
	fs.Var(&envs, "env", "NAME=PATH, repeatable, at least 2 required")
	rulesPath := fs.String("rules", "", "path to a rules YAML file")
	out := fs.String("out", "", "snapshot output file (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("--out is required")
	}

	parsedEnvs, err := envs.parse()
	if err != nil {
		return err
	}
	r, err := loadRules(*rulesPath)
	if err != nil {
		return err
	}

	snap := snapshot.Build(parsedEnvs, r)
	if err := snapshot.Save(*out, snap); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "snapshot written to %s (%d keys, %d environments)\n", *out, len(snap.Entries), len(snap.Environments))
	return nil
}

func runDrift(args []string, stdout io.Writer) (int, error) {
	fs := flag.NewFlagSet("drift", flag.ContinueOnError)
	var envs envFlags
	fs.Var(&envs, "env", "NAME=PATH, repeatable, at least 2 required")
	rulesPath := fs.String("rules", "", "path to a rules YAML file")
	snapPath := fs.String("snapshot", "", "path to a previously saved snapshot (required)")
	format := fs.String("format", "table", "output format: table|json")
	exitZero := fs.Bool("exit-zero", false, "always exit 0, even if drift is found")
	if err := fs.Parse(args); err != nil {
		return 0, err
	}
	if *snapPath == "" {
		return 0, fmt.Errorf("--snapshot is required")
	}
	if *format != "table" && *format != "json" {
		return 0, fmt.Errorf("unknown --format %q (want table or json)", *format)
	}

	parsedEnvs, err := envs.parse()
	if err != nil {
		return 0, err
	}
	r, err := loadRules(*rulesPath)
	if err != nil {
		return 0, err
	}

	oldSnap, err := snapshot.Load(*snapPath)
	if err != nil {
		return 0, err
	}
	newSnap := snapshot.Build(parsedEnvs, r)
	drifted := snapshot.WithoutIgnored(snapshot.Diff(oldSnap, newSnap), r)

	sort.Slice(drifted, func(i, j int) bool {
		if drifted[i].Key != drifted[j].Key {
			return drifted[i].Key < drifted[j].Key
		}
		return drifted[i].Env < drifted[j].Env
	})

	if *format == "table" {
		printDriftTable(stdout, drifted, oldSnap.CreatedAt.String())
	} else if err := printDriftJSON(stdout, drifted); err != nil {
		return 0, err
	}

	if !*exitZero && len(drifted) > 0 {
		return 1, nil
	}
	return 0, nil
}

func printDriftTable(w io.Writer, entries []snapshot.DriftEntry, since string) {
	fmt.Fprintf(w, "drift since snapshot taken at %s:\n\n", since)
	if len(entries) == 0 {
		fmt.Fprintln(w, "no drift detected")
		return
	}
	for _, e := range entries {
		note := ""
		if e.Secret {
			note = " (secret value change, hash-based)"
		}
		fmt.Fprintf(w, "  %-10s %-30s %s%s\n", e.Env, e.Key, e.Change, note)
	}
	fmt.Fprintf(w, "\n%d newly diverged key(s)\n", len(entries))
}

func printDriftJSON(w io.Writer, entries []snapshot.DriftEntry) error {
	out := struct {
		Drifted []snapshot.DriftEntry `json:"drifted"`
		Count   int                   `json:"count"`
	}{Drifted: entries, Count: len(entries)}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
