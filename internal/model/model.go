// Package model defines the shared data types used to represent parsed
// configuration values across every supported file format.
package model

import "sort"

// Kind describes the native type a configuration value was parsed as.
// It is used to detect "type-differs" drift: the same key represented as
// a string in one environment and a boolean or number in another.
type Kind string

const (
	KindString Kind = "string"
	KindBool   Kind = "bool"
	KindNumber Kind = "number"
	KindNull   Kind = "null"
)

// Value is a single flattened configuration value.
type Value struct {
	// Raw is the string representation of the value, used for display,
	// comparison and hashing.
	Raw string
	// Kind is the native type the value was parsed with.
	Kind Kind
}

// IsEmpty reports whether the value is considered "empty": a null value,
// or a string value with zero length.
func (v Value) IsEmpty() bool {
	if v.Kind == KindNull {
		return true
	}
	return v.Kind == KindString && v.Raw == ""
}

// Config is a flattened set of configuration key/value pairs. Nested
// structures (YAML maps, JSON objects, TOML tables, array elements) are
// flattened to dotted paths, e.g. "database.host" or "servers.0.port".
type Config map[string]Value

// Environment is a named, parsed configuration source.
type Environment struct {
	Name   string
	Path   string
	Format string
	Config Config
}

// SortedKeys returns the config's keys in a deterministic, sorted order.
func (c Config) SortedKeys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// UnionKeys returns the sorted union of all keys present across the given
// environments.
func UnionKeys(envs []Environment) []string {
	seen := make(map[string]struct{})
	for _, e := range envs {
		for k := range e.Config {
			seen[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
