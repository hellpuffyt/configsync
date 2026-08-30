// Package rules loads and applies drift rules: which keys are expected to
// differ between environments, which must be identical everywhere, which
// should be ignored entirely, and which hold secret values that must never
// be printed.
package rules

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Rules holds the glob patterns loaded from a rules file. Patterns use '*'
// as a wildcard and are matched case-insensitively against both the full
// dotted key and its final path segment.
type Rules struct {
	ExpectedVariance []string `yaml:"expected_variance"`
	MustMatch        []string `yaml:"must_match"`
	Ignore           []string `yaml:"ignore"`
	SecretPatterns   []string `yaml:"secret_patterns"`
}

// defaultSecretPatterns are always active, even with no rules file, so a
// misconfigured or missing rules file can never turn off secret redaction.
var defaultSecretPatterns = []string{
	"*PASSWORD*",
	"*PASSWD*",
	"*_PASS",
	"*PWD*",
	"*SECRET*",
	"*TOKEN*",
	"*_KEY",
	"*APIKEY*",
	"*API_KEY*",
	"*CREDENTIAL*",
	"*PRIVATE*",
	"*_CERT",
	"*_DSN",
}

// Default returns an empty rule set that still carries the built-in secret
// patterns, for use when no --rules file is given.
func Default() *Rules {
	return &Rules{}
}

// Load reads and parses a YAML rules file.
func Load(path string) (*Rules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Rules
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// IsIgnored reports whether key matches an ignore pattern.
func (r *Rules) IsIgnored(key string) bool {
	if r == nil {
		return false
	}
	return r.matchesAny(key, r.Ignore)
}

// IsExpectedVariance reports whether key matches an expected-variance
// pattern (URLs, hostnames, credentials, ports, etc).
func (r *Rules) IsExpectedVariance(key string) bool {
	if r == nil {
		return false
	}
	return r.matchesAny(key, r.ExpectedVariance)
}

// IsMustMatch reports whether key matches a must-match pattern (feature
// flags, timeouts, anything that should be identical in every environment).
func (r *Rules) IsMustMatch(key string) bool {
	if r == nil {
		return false
	}
	return r.matchesAny(key, r.MustMatch)
}

// IsSecret reports whether key matches a secret pattern, either from the
// rules file or the built-in defaults. Secret keys never have their values
// printed in any output. A nil Rules still applies the built-in defaults.
func (r *Rules) IsSecret(key string) bool {
	if matchesAnyPattern(key, defaultSecretPatterns) {
		return true
	}
	if r == nil {
		return false
	}
	return r.matchesAny(key, r.SecretPatterns)
}

func (r *Rules) matchesAny(key string, patterns []string) bool {
	return matchesAnyPattern(key, patterns)
}

func matchesAnyPattern(key string, patterns []string) bool {
	leaf := key
	if idx := strings.LastIndex(key, "."); idx >= 0 {
		leaf = key[idx+1:]
	}
	for _, p := range patterns {
		if Match(p, key) || Match(p, leaf) {
			return true
		}
	}
	return false
}

// Match reports whether s matches the glob pattern, case-insensitively.
// The only wildcard supported is '*', matching zero or more characters.
func Match(pattern, s string) bool {
	pattern = strings.ToUpper(pattern)
	s = strings.ToUpper(s)
	return globMatch(pattern, s)
}

func globMatch(pattern, s string) bool {
	segments := strings.Split(pattern, "*")
	if len(segments) == 1 {
		return pattern == s
	}

	if !strings.HasPrefix(s, segments[0]) {
		return false
	}
	s = s[len(segments[0]):]

	if !strings.HasSuffix(pattern, "*") {
		last := segments[len(segments)-1]
		if !strings.HasSuffix(s, last) {
			return false
		}
		s = s[:len(s)-len(last)]
		segments = segments[:len(segments)-1]
	}

	for _, seg := range segments[1:] {
		if seg == "" {
			continue
		}
		idx := strings.Index(s, seg)
		if idx < 0 {
			return false
		}
		s = s[idx+len(seg):]
	}
	return true
}
