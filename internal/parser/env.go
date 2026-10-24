package parser

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/hellpuffyt/configsync/internal/model"
)

// ParseEnv parses the contents of a .env-style file: KEY=VALUE lines,
// optional "export " prefix, single- and double-quoted values (with
// backslash escapes inside double quotes), '#' comments, and blank lines.
func ParseEnv(data []byte) (model.Config, error) {
	out := make(model.Config)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		line = strings.TrimSpace(line)

		eq := strings.Index(line, "=")
		if eq < 0 {
			return nil, fmt.Errorf("line %d: missing '='", lineNo)
		}
		key := strings.TrimSpace(line[:eq])
		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", lineNo)
		}
		rawValue := strings.TrimSpace(line[eq+1:])
		value := parseEnvValue(rawValue)
		out[key] = model.Value{Raw: value, Kind: model.KindString}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parseEnvValue(raw string) string {
	if len(raw) >= 2 && raw[0] == '"' {
		if end := findUnescapedQuote(raw[1:], '"'); end >= 0 {
			inner := raw[1 : 1+end]
			return unescapeDouble(inner)
		}
	}
	if len(raw) >= 2 && raw[0] == '\'' {
		if end := strings.IndexByte(raw[1:], '\''); end >= 0 {
			return raw[1 : 1+end]
		}
	}
	// Unquoted: strip a trailing inline comment ("KEY=value # comment").
	if idx := strings.Index(raw, " #"); idx >= 0 {
		raw = strings.TrimSpace(raw[:idx])
	}
	return raw
}

func findUnescapedQuote(s string, quote byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == quote {
			return i
		}
	}
	return -1
}

// unescapeDouble decodes the escapes allowed inside a double-quoted .env
// value: \n \t \r \" and \\. Any other backslash is kept as written.
// (A strings.Replacer cannot do this: it decoded "\\n" as a backslash
// followed by a newline.)
func unescapeDouble(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		switch s[i+1] {
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
			b.WriteByte('\\')
			b.WriteByte(s[i+1])
		}
		i++
	}
	return b.String()
}
