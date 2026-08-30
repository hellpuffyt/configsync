package report

import (
	"encoding/json"
	"io"
	"time"

	"github.com/prabeshsharma/configsync/internal/diff"
)

// JSONCell is one environment's redacted view of a key, for JSON output.
type JSONCell struct {
	Present  bool   `json:"present"`
	Value    string `json:"value,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Empty    bool   `json:"empty,omitempty"`
	Redacted bool   `json:"redacted,omitempty"`
}

// JSONEntry is the JSON representation of a single diff.Entry.
type JSONEntry struct {
	Key                string              `json:"key"`
	Classification     string              `json:"classification"`
	BaseClassification string              `json:"base_classification"`
	Unexpected         bool                `json:"unexpected"`
	MustMatchViolation bool                `json:"must_match_violation,omitempty"`
	Secret             bool                `json:"secret,omitempty"`
	Values             map[string]JSONCell `json:"values"`
}

// Summary counts entries per classification.
type Summary struct {
	TotalKeys        int `json:"total_keys"`
	Unexpected       int `json:"unexpected"`
	Missing          int `json:"missing"`
	ValueDiffers     int `json:"value_differs"`
	EmptyIn          int `json:"empty_in"`
	TypeDiffers      int `json:"type_differs"`
	ExpectedVariance int `json:"expected_variance"`
	OK               int `json:"ok"`
}

// Document is the top-level JSON report.
type Document struct {
	GeneratedAt  time.Time   `json:"generated_at"`
	Environments []string    `json:"environments"`
	Summary      Summary     `json:"summary"`
	Entries      []JSONEntry `json:"entries"`
}

// BuildDocument converts a diff.Matrix into a JSON-serializable Document.
func BuildDocument(m diff.Matrix) Document {
	doc := Document{
		GeneratedAt:  time.Now().UTC(),
		Environments: m.Environments,
	}
	for _, e := range sortedEntries(m.Entries) {
		je := JSONEntry{
			Key:                e.Key,
			Classification:     string(e.Classification),
			BaseClassification: string(e.BaseClassification),
			Unexpected:         e.Unexpected,
			MustMatchViolation: e.MustMatchViolation,
			Secret:             e.Secret,
			Values:             make(map[string]JSONCell, len(m.Environments)),
		}
		for _, env := range m.Environments {
			c := e.Cells[env]
			jc := JSONCell{Present: c.Present}
			if c.Present {
				jc.Kind = string(c.Value.Kind)
				jc.Empty = c.Value.IsEmpty()
				if e.Secret {
					jc.Redacted = true
				} else {
					jc.Value = c.Value.Raw
				}
			}
			je.Values[env] = jc
		}
		doc.Entries = append(doc.Entries, je)

		doc.Summary.TotalKeys++
		switch e.Classification {
		case diff.OK:
			doc.Summary.OK++
		case diff.Missing:
			doc.Summary.Missing++
		case diff.ValueDiffers:
			doc.Summary.ValueDiffers++
		case diff.EmptyIn:
			doc.Summary.EmptyIn++
		case diff.TypeDiffers:
			doc.Summary.TypeDiffers++
		case diff.ExpectedVariance:
			doc.Summary.ExpectedVariance++
		}
		if e.Unexpected {
			doc.Summary.Unexpected++
		}
	}
	return doc
}

// JSON writes the matrix as indented JSON to w.
func JSON(w io.Writer, m diff.Matrix) error {
	doc := BuildDocument(m)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
