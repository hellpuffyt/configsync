package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hellpuffyt/configsync/internal/model"
)

// ParseTOML parses a practical subset of TOML: [table] and [[array.of.tables]]
// headers, dotted keys, quoted and bare keys, string/bool/int (decimal, hex,
// octal, binary)/float scalars, datetimes (kept as strings), and single-line
// inline arrays of scalars. Inline tables, multi-line strings and multi-line
// arrays are not supported and are rejected with an error rather than
// misread; so are duplicate keys.
func ParseTOML(data []byte) (model.Config, error) {
	root := make(map[string]interface{})
	current := root

	lines := strings.Split(string(data), "\n")
	for lineNo, raw := range lines {
		line := strings.TrimSpace(stripTOMLComment(raw))
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[[") && strings.HasSuffix(line, "]]") {
			path := strings.TrimSpace(line[2 : len(line)-2])
			tbl, err := appendTableArray(root, splitTOMLPath(path))
			if err != nil {
				return nil, fmt.Errorf("toml line %d: %w", lineNo+1, err)
			}
			current = tbl
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			path := strings.TrimSpace(line[1 : len(line)-1])
			tbl, err := ensureTable(root, splitTOMLPath(path))
			if err != nil {
				return nil, fmt.Errorf("toml line %d: %w", lineNo+1, err)
			}
			current = tbl
			continue
		}

		eq := indexUnquoted(line, '=')
		if eq < 0 {
			return nil, fmt.Errorf("toml line %d: expected 'key = value'", lineNo+1)
		}
		keyPart := strings.TrimSpace(line[:eq])
		valPart := strings.TrimSpace(line[eq+1:])

		value, err := parseTOMLValue(valPart)
		if err != nil {
			return nil, fmt.Errorf("toml line %d: %w", lineNo+1, err)
		}
		if err := setDotted(current, splitTOMLPath(keyPart), value); err != nil {
			return nil, fmt.Errorf("toml line %d: %w", lineNo+1, err)
		}
	}

	out := make(model.Config)
	Flatten(root, "", out)
	return out, nil
}

// quoteState tracks whether a left-to-right scan is inside a TOML string.
// Basic ("...") strings honour backslash escapes, so \" does not end them;
// literal ('...') strings have no escapes.
type quoteState struct{ inSingle, inDouble, escaped bool }

// step advances over c and reports whether c is outside any string.
func (q *quoteState) step(c byte) bool {
	switch {
	case q.inDouble:
		if q.escaped {
			q.escaped = false
		} else if c == '\\' {
			q.escaped = true
		} else if c == '"' {
			q.inDouble = false
		}
		return false
	case q.inSingle:
		if c == '\'' {
			q.inSingle = false
		}
		return false
	case c == '"':
		q.inDouble = true
		return false
	case c == '\'':
		q.inSingle = true
		return false
	}
	return true
}

func stripTOMLComment(line string) string {
	var q quoteState
	for i := 0; i < len(line); i++ {
		if q.step(line[i]) && line[i] == '#' {
			return line[:i]
		}
	}
	return line
}

func indexUnquoted(s string, target byte) int {
	var q quoteState
	for i := 0; i < len(s); i++ {
		if q.step(s[i]) && s[i] == target {
			return i
		}
	}
	return -1
}

func splitTOMLPath(s string) []string {
	var parts []string
	var buf strings.Builder
	inSingle, inDouble := false, false
	flush := func() {
		parts = append(parts, strings.TrimSpace(unquoteTOMLKey(buf.String())))
		buf.Reset()
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
			buf.WriteByte(c)
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
			buf.WriteByte(c)
		case '.':
			if inSingle || inDouble {
				buf.WriteByte(c)
			} else {
				flush()
			}
		default:
			buf.WriteByte(c)
		}
	}
	flush()
	return parts
}

func unquoteTOMLKey(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

func ensureTable(root map[string]interface{}, path []string) (map[string]interface{}, error) {
	cur := root
	for _, p := range path {
		next, ok := cur[p]
		if !ok {
			m := make(map[string]interface{})
			cur[p] = m
			cur = m
			continue
		}
		switch v := next.(type) {
		case map[string]interface{}:
			cur = v
		case []interface{}:
			if len(v) == 0 {
				return nil, fmt.Errorf("cannot descend into empty array %q", p)
			}
			last, ok := v[len(v)-1].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("path %q is not a table", p)
			}
			cur = last
		default:
			return nil, fmt.Errorf("key %q already has a scalar value", p)
		}
	}
	return cur, nil
}

func appendTableArray(root map[string]interface{}, path []string) (map[string]interface{}, error) {
	parent, err := ensureTable(root, path[:len(path)-1])
	if err != nil {
		return nil, err
	}
	last := path[len(path)-1]
	newTbl := make(map[string]interface{})
	existing, ok := parent[last]
	if !ok {
		parent[last] = []interface{}{newTbl}
		return newTbl, nil
	}
	arr, ok := existing.([]interface{})
	if !ok {
		return nil, fmt.Errorf("key %q is not an array of tables", last)
	}
	parent[last] = append(arr, newTbl)
	return newTbl, nil
}

func setDotted(tbl map[string]interface{}, path []string, value interface{}) error {
	cur := tbl
	for _, p := range path[:len(path)-1] {
		next, ok := cur[p]
		if !ok {
			m := make(map[string]interface{})
			cur[p] = m
			cur = m
			continue
		}
		m, ok := next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("key %q already has a scalar value", p)
		}
		cur = m
	}
	last := path[len(path)-1]
	if _, exists := cur[last]; exists {
		// TOML forbids defining a key twice; silently keeping the last
		// value (or replacing a whole table) would hide the conflict.
		return fmt.Errorf("duplicate key %q", strings.Join(path, "."))
	}
	cur[last] = value
	return nil
}

func parseTOMLValue(s string) (interface{}, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty value")
	}
	switch {
	case s == "true":
		return true, nil
	case s == "false":
		return false, nil
	case strings.HasPrefix(s, `"""`) || strings.HasPrefix(s, "'''"):
		return nil, fmt.Errorf("multi-line strings are not supported")
	case strings.HasPrefix(s, "{"):
		return nil, fmt.Errorf("inline tables are not supported (use a [table] header)")
	case strings.HasPrefix(s, `"`):
		return parseTOMLBasicString(s)
	case strings.HasPrefix(s, `'`):
		if len(s) < 2 || s[len(s)-1] != '\'' {
			return nil, fmt.Errorf("unterminated literal string")
		}
		return s[1 : len(s)-1], nil
	case strings.HasPrefix(s, "["):
		return parseTOMLArray(s)
	default:
		if tomlDateTime.MatchString(s) {
			return s, nil // datetimes are compared as written
		}
		clean := strings.ReplaceAll(s, "_", "")
		for prefix, b := range map[string]int{"0x": 16, "0o": 8, "0b": 2} {
			if strings.HasPrefix(clean, prefix) {
				i, err := strconv.ParseInt(clean[2:], b, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid integer")
				}
				return i, nil
			}
		}
		if i, err := strconv.ParseInt(clean, 10, 64); err == nil {
			return i, nil
		}
		if tomlFloat.MatchString(clean) {
			if f, err := strconv.ParseFloat(clean, 64); err == nil {
				return f, nil
			}
		}
		// Anything else (e.g. an unquoted word) is not valid TOML; it used
		// to be accepted as a string, hiding typos and unsupported syntax.
		return nil, fmt.Errorf("unsupported or invalid value")
	}
}

var (
	tomlDateTime = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}([Tt ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?([Zz]|[+-]\d{2}:\d{2})?)?|\d{2}:\d{2}:\d{2}(\.\d+)?)$`)
	tomlFloat    = regexp.MustCompile(`^[+-]?(\d+(\.\d+)?([eE][+-]?\d+)?|inf|nan)$`)
)

func parseTOMLBasicString(s string) (string, error) {
	if len(s) < 2 || s[len(s)-1] != '"' {
		return "", fmt.Errorf("unterminated string")
	}
	inner := s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		if inner[i] == '\\' && i+1 < len(inner) {
			switch inner[i+1] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte(inner[i+1])
			}
			i++
			continue
		}
		b.WriteByte(inner[i])
	}
	return b.String(), nil
}

func parseTOMLArray(s string) ([]interface{}, error) {
	if !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("unterminated array")
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return []interface{}{}, nil
	}
	var elems []interface{}
	for _, part := range splitTOMLArrayElems(inner) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := parseTOMLValue(part)
		if err != nil {
			return nil, err
		}
		elems = append(elems, v)
	}
	return elems, nil
}

func splitTOMLArrayElems(s string) []string {
	var parts []string
	depth := 0
	var q quoteState
	start := 0
	for i := 0; i < len(s); i++ {
		if !q.step(s[i]) {
			continue
		}
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}
