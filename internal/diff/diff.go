// Package diff compares two or more parsed environments and classifies
// every difference between them.
package diff

import (
	"github.com/prabeshsharma/configsync/internal/model"
	"github.com/prabeshsharma/configsync/internal/rules"
)

// Classification identifies the kind of drift found for a single key.
type Classification string

const (
	// OK means the key is present with an identical, same-typed value in
	// every environment (or was equally absent — though absent-everywhere
	// keys never appear since they're not in the key union).
	OK Classification = "ok"
	// Missing means the key is absent in at least one environment.
	Missing Classification = "missing"
	// ValueDiffers means the key is present everywhere with differing values.
	ValueDiffers Classification = "value-differs"
	// EmptyIn means the key is present everywhere but empty in at least one.
	EmptyIn Classification = "empty-in"
	// TypeDiffers means the key's native type differs across environments,
	// e.g. the string "true" in one environment and the boolean true in another.
	TypeDiffers Classification = "type-differs"
	// ExpectedVariance means the key matched an expected-variance rule, so
	// what would otherwise be reported drift is not considered unexpected.
	ExpectedVariance Classification = "expected-variance"
)

// Cell is one environment's view of a single key.
type Cell struct {
	Present bool
	Value   model.Value
}

// Entry is the full drift report for a single key across all environments.
type Entry struct {
	Key                string
	Classification     Classification
	BaseClassification Classification // the classification before rules were applied
	Unexpected         bool           // true if this should fail a CI gate
	MustMatchViolation bool
	Secret             bool
	Cells              map[string]Cell
}

// Matrix is the full comparison result.
type Matrix struct {
	Environments []string
	Entries      []Entry
}

// UnexpectedCount returns the number of entries flagged as unexpected drift.
func (m Matrix) UnexpectedCount() int {
	n := 0
	for _, e := range m.Entries {
		if e.Unexpected {
			n++
		}
	}
	return n
}

// Compare builds the full drift matrix for the given environments, applying
// the supplied rules to classify and suppress expected differences.
func Compare(envs []model.Environment, r *rules.Rules) Matrix {
	if r == nil {
		r = rules.Default()
	}
	names := make([]string, len(envs))
	for i, e := range envs {
		names[i] = e.Name
	}

	m := Matrix{Environments: names}
	for _, key := range model.UnionKeys(envs) {
		if r.IsIgnored(key) {
			continue
		}
		m.Entries = append(m.Entries, buildEntry(key, envs, r))
	}
	return m
}

func buildEntry(key string, envs []model.Environment, r *rules.Rules) Entry {
	cells := make(map[string]Cell, len(envs))
	allPresent := true
	kinds := make(map[model.Kind]bool)
	values := make(map[string]bool)
	anyEmpty := false
	emptyCount := 0

	for _, e := range envs {
		v, present := e.Config[key]
		cells[e.Name] = Cell{Present: present, Value: v}
		if !present {
			allPresent = false
			continue
		}
		kinds[v.Kind] = true
		values[v.Raw] = true
		if v.IsEmpty() {
			anyEmpty = true
			emptyCount++
		}
	}

	base := OK
	switch {
	case !allPresent:
		base = Missing
	case len(kinds) > 1:
		base = TypeDiffers
	case anyEmpty && emptyCount < len(envs):
		base = EmptyIn
	case len(values) > 1:
		base = ValueDiffers
	}

	entry := Entry{
		Key:                key,
		BaseClassification: base,
		Cells:              cells,
		Secret:             r.IsSecret(key),
	}

	mustMatch := r.IsMustMatch(key)
	expected := r.IsExpectedVariance(key)

	switch {
	case base == OK:
		entry.Classification = OK
		entry.Unexpected = false
	case mustMatch:
		entry.Classification = base
		entry.Unexpected = true
		entry.MustMatchViolation = true
	case base != TypeDiffers && expected:
		entry.Classification = ExpectedVariance
		entry.Unexpected = false
	default:
		entry.Classification = base
		entry.Unexpected = true
	}

	return entry
}
