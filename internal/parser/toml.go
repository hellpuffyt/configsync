package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/prabeshsharma/configsync/internal/model"
)

// ParseTOML parses a practical subset of TOML: [table] and [[array.of.tables]]
// headers, dotted keys, quoted and bare keys, string/bool/int/float scalars,
// and single-line inline arrays of scalars. It does not support multi-line
// arrays, inline tables, or TOML datetimes.
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

func stripTOMLComment(line string) string {
	inSingle, inDouble := false, false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '#':
			if !inSingle && !inDouble {
				return line[:i]
			}
		}
	}
	return line
}

func indexUnquoted(s string, target byte) int {
	inSingle, inDouble := false, false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		default:
			if s[i] == target && !inSingle && !inDouble {
				return i
			}
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
	cur[path[len(path)-1]] = value
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
	case strings.HasPrefix(s, `"`):
		return parseTOMLBasicString(s)
	case strings.HasPrefix(s, `'`):
		if len(s) < 2 || s[len(s)-1] != '\'' {
			return nil, fmt.Errorf("unterminated literal string: %s", s)
		}
		return s[1 : len(s)-1], nil
	case strings.HasPrefix(s, "["):
		return parseTOMLArray(s)
	default:
		clean := strings.ReplaceAll(s, "_", "")
		if i, err := strconv.ParseInt(clean, 10, 64); err == nil {
			return i, nil
		}
		if f, err := strconv.ParseFloat(clean, 64); err == nil {
			return f, nil
		}
		// Fall back to a bare/unquoted string (dates, unknown literals, etc).
		return s, nil
	}
}

func parseTOMLBasicString(s string) (string, error) {
	if len(s) < 2 || s[len(s)-1] != '"' {
		return "", fmt.Errorf("unterminated string: %s", s)
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
		return nil, fmt.Errorf("unterminated array: %s", s)
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
	inSingle, inDouble := false, false
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '[':
			if !inSingle && !inDouble {
				depth++
			}
		case ']':
			if !inSingle && !inDouble {
				depth--
			}
		case ',':
			if !inSingle && !inDouble && depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}
