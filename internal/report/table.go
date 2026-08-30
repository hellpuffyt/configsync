// Package report renders a diff.Matrix as a human-readable table or as
// JSON, redacting secret values in both.
package report

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/prabeshsharma/configsync/internal/diff"
)

// cellText renders a single matrix cell for display, honoring secret
// redaction so that a secret value is never written to any output stream.
func cellText(c diff.Cell, secret bool) string {
	if !c.Present {
		return "<missing>"
	}
	if secret {
		return "[REDACTED]"
	}
	if c.Value.IsEmpty() {
		return "<empty>"
	}
	return c.Value.Raw
}

// Table writes a human-readable matrix table to w. When diffOnly is true,
// rows classified "ok" are omitted.
func Table(w io.Writer, m diff.Matrix, diffOnly bool) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)

	header := []string{"KEY"}
	header = append(header, m.Environments...)
	header = append(header, "STATUS")
	fmt.Fprintln(tw, strings.Join(header, "\t"))

	shown := 0
	for _, e := range sortedEntries(m.Entries) {
		if diffOnly && e.Classification == diff.OK {
			continue
		}
		row := []string{e.Key}
		for _, env := range m.Environments {
			row = append(row, cellText(e.Cells[env], e.Secret))
		}
		status := string(e.Classification)
		if e.MustMatchViolation {
			status += " (must-match)"
		}
		row = append(row, status)
		fmt.Fprintln(tw, strings.Join(row, "\t"))
		shown++
	}
	tw.Flush()

	fmt.Fprintln(w)
	fmt.Fprintf(w, "%d key(s) shown, %d unexpected drift\n", shown, m.UnexpectedCount())
}

func sortedEntries(entries []diff.Entry) []diff.Entry {
	out := make([]diff.Entry, len(entries))
	copy(out, entries)
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
