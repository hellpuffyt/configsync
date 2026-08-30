// Command configsync detects configuration drift between two or more
// environments (dev/staging/prod), classifying each difference so the
// important distinctions -- a key that differs, a key that is missing, and
// a key that is merely empty -- are never conflated the way a plain diff
// conflates them.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/prabeshsharma/configsync/internal/diff"
	"github.com/prabeshsharma/configsync/internal/model"
	"github.com/prabeshsharma/configsync/internal/parser"
	"github.com/prabeshsharma/configsync/internal/report"
	"github.com/prabeshsharma/configsync/internal/rules"
	"github.com/prabeshsharma/configsync/internal/snapshot"
)

const version = "0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "configsync:", err)
		os.Exit(2)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return fmt.Errorf("missing command")
	}

	switch args[0] {
	case "compare":
		return runCompare(args[1:])
	case "snapshot":
		return runSnapshot(args[1:])
	case "drift":
		return runDrift(args[1:])
	case "version":
		fmt.Println("configsync", version)
		return nil
	case "-h", "--help", "help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
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
	for _, ent := range e.entries {
		name, path, ok := strings.Cut(ent, "=")
		if !ok || name == "" || path == "" {
			return nil, fmt.Errorf("invalid --env value %q, expected NAME=PATH", ent)
		}
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

func runCompare(args []string) error {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	var envs envFlags
	fs.Var(&envs, "env", "NAME=PATH, repeatable, at least 2 required")
	rulesPath := fs.String("rules", "", "path to a rules YAML file")
	format := fs.String("format", "table", "output format: table|json")
	all := fs.Bool("all", false, "show all keys, including keys that match everywhere (table format only)")
	exitZero := fs.Bool("exit-zero", false, "always exit 0, even if unexpected drift is found")
	if err := fs.Parse(args); err != nil {
		return err
	}

	parsedEnvs, err := envs.parse()
	if err != nil {
		return err
	}
	r, err := loadRules(*rulesPath)
	if err != nil {
		return err
	}

	matrix := diff.Compare(parsedEnvs, r)
	if err := writeReport(os.Stdout, matrix, *format, !*all); err != nil {
		return err
	}

	if !*exitZero && matrix.UnexpectedCount() > 0 {
		os.Exit(1)
	}
	return nil
}

func writeReport(w *os.File, matrix diff.Matrix, format string, diffOnly bool) error {
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

func runSnapshot(args []string) error {
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
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
	fmt.Printf("snapshot written to %s (%d keys, %d environments)\n", *out, len(snap.Entries), len(snap.Environments))
	return nil
}

func runDrift(args []string) error {
	fs := flag.NewFlagSet("drift", flag.ExitOnError)
	var envs envFlags
	fs.Var(&envs, "env", "NAME=PATH, repeatable, at least 2 required")
	rulesPath := fs.String("rules", "", "path to a rules YAML file")
	snapPath := fs.String("snapshot", "", "path to a previously saved snapshot (required)")
	format := fs.String("format", "table", "output format: table|json")
	exitZero := fs.Bool("exit-zero", false, "always exit 0, even if drift is found")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *snapPath == "" {
		return fmt.Errorf("--snapshot is required")
	}

	parsedEnvs, err := envs.parse()
	if err != nil {
		return err
	}
	r, err := loadRules(*rulesPath)
	if err != nil {
		return err
	}

	oldSnap, err := snapshot.Load(*snapPath)
	if err != nil {
		return err
	}
	newSnap := snapshot.Build(parsedEnvs, r)
	drifted := snapshot.Diff(oldSnap, newSnap)

	sort.Slice(drifted, func(i, j int) bool {
		if drifted[i].Key != drifted[j].Key {
			return drifted[i].Key < drifted[j].Key
		}
		return drifted[i].Env < drifted[j].Env
	})

	switch *format {
	case "table":
		printDriftTable(drifted, oldSnap.CreatedAt.String())
	case "json":
		if err := printDriftJSON(drifted); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown --format %q (want table or json)", *format)
	}

	if !*exitZero && len(drifted) > 0 {
		os.Exit(1)
	}
	return nil
}

func printDriftTable(entries []snapshot.DriftEntry, since string) {
	fmt.Printf("drift since snapshot taken at %s:\n\n", since)
	if len(entries) == 0 {
		fmt.Println("no drift detected")
		return
	}
	for _, e := range entries {
		note := ""
		if e.Secret {
			note = " (secret value change, hash-based)"
		}
		fmt.Printf("  %-10s %-30s %s%s\n", e.Env, e.Key, e.Change, note)
	}
	fmt.Printf("\n%d newly diverged key(s)\n", len(entries))
}

func printDriftJSON(entries []snapshot.DriftEntry) error {
	out := struct {
		Drifted []snapshot.DriftEntry `json:"drifted"`
		Count   int                   `json:"count"`
	}{Drifted: entries, Count: len(entries)}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
