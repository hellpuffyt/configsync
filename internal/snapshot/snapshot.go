// Package snapshot saves point-in-time configuration state and compares a
// later run against it to report drift that has accumulated over time.
// Secret values are never stored in plaintext: only a SHA-256 hash is kept,
// so drift detection still works without the snapshot file itself becoming
// a leak.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/prabeshsharma/configsync/internal/model"
	"github.com/prabeshsharma/configsync/internal/rules"
)

// Entry is one environment's recorded state for a single key.
type Entry struct {
	Present bool   `json:"present"`
	Kind    string `json:"kind,omitempty"`
	Value   string `json:"value,omitempty"`
	Hash    string `json:"hash,omitempty"`
	Secret  bool   `json:"secret,omitempty"`
}

// Snapshot is the on-disk drift baseline.
type Snapshot struct {
	CreatedAt    time.Time                   `json:"created_at"`
	Environments []string                    `json:"environments"`
	Entries      map[string]map[string]Entry `json:"entries"`
}

// Build captures the current state of the given environments into a Snapshot.
func Build(envs []model.Environment, r *rules.Rules) Snapshot {
	if r == nil {
		r = rules.Default()
	}
	names := make([]string, len(envs))
	for i, e := range envs {
		names[i] = e.Name
	}
	snap := Snapshot{
		CreatedAt:    time.Now().UTC(),
		Environments: names,
		Entries:      make(map[string]map[string]Entry),
	}
	for _, key := range model.UnionKeys(envs) {
		secret := r.IsSecret(key)
		perEnv := make(map[string]Entry, len(envs))
		for _, e := range envs {
			v, present := e.Config[key]
			entry := Entry{Present: present, Secret: secret}
			if present {
				entry.Kind = string(v.Kind)
				if secret {
					entry.Hash = hashValue(v.Raw)
				} else {
					entry.Value = v.Raw
				}
			}
			perEnv[e.Name] = entry
		}
		snap.Entries[key] = perEnv
	}
	return snap
}

func hashValue(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Save writes the snapshot to path as indented JSON.
func Save(path string, snap Snapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Load reads a snapshot from path.
func Load(path string) (Snapshot, error) {
	var snap Snapshot
	data, err := os.ReadFile(path)
	if err != nil {
		return snap, err
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return snap, fmt.Errorf("parsing snapshot %q: %w", path, err)
	}
	return snap, nil
}

// ChangeKind identifies how a key's state changed between two snapshots.
type ChangeKind string

const (
	BecamePresent ChangeKind = "became-present"
	BecameMissing ChangeKind = "became-missing"
	ValueChanged  ChangeKind = "value-changed"
)

// DriftEntry describes one key/environment change detected between two
// snapshots taken at different times.
type DriftEntry struct {
	Key    string     `json:"key"`
	Env    string     `json:"env"`
	Change ChangeKind `json:"change"`
	Secret bool       `json:"secret,omitempty"`
}

// Diff compares an older snapshot against a newer one and returns every
// key/environment pair whose state changed. Secret values are compared by
// hash only, so a changed secret is reported without ever revealing it.
func Diff(oldSnap, newSnap Snapshot) []DriftEntry {
	var out []DriftEntry

	keys := make(map[string]struct{})
	for k := range oldSnap.Entries {
		keys[k] = struct{}{}
	}
	for k := range newSnap.Entries {
		keys[k] = struct{}{}
	}
	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	envs := make(map[string]struct{})
	for _, e := range oldSnap.Environments {
		envs[e] = struct{}{}
	}
	for _, e := range newSnap.Environments {
		envs[e] = struct{}{}
	}
	sortedEnvs := make([]string, 0, len(envs))
	for e := range envs {
		sortedEnvs = append(sortedEnvs, e)
	}
	sort.Strings(sortedEnvs)

	for _, key := range sortedKeys {
		oldPerEnv := oldSnap.Entries[key]
		newPerEnv := newSnap.Entries[key]
		for _, env := range sortedEnvs {
			oe, oldOK := oldPerEnv[env]
			ne, newOK := newPerEnv[env]

			switch {
			case !oldOK && !newOK:
				continue
			case (!oldOK || !oe.Present) && newOK && ne.Present:
				out = append(out, DriftEntry{Key: key, Env: env, Change: BecamePresent, Secret: ne.Secret})
			case oldOK && oe.Present && (!newOK || !ne.Present):
				out = append(out, DriftEntry{Key: key, Env: env, Change: BecameMissing, Secret: oe.Secret})
			case oldOK && newOK && oe.Present && ne.Present:
				if oe.Secret || ne.Secret {
					if oe.Hash != ne.Hash {
						out = append(out, DriftEntry{Key: key, Env: env, Change: ValueChanged, Secret: true})
					}
				} else if oe.Value != ne.Value || oe.Kind != ne.Kind {
					out = append(out, DriftEntry{Key: key, Env: env, Change: ValueChanged})
				}
			}
		}
	}
	return out
}
