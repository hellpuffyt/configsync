package parser

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/prabeshsharma/configsync/internal/model"
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
			return nil, fmt.Errorf("line %d: missing '=' in %q", lineNo, line)
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

func unescapeDouble(s string) string {
	replacer := strings.NewReplacer(
		`\n`, "\n",
		`\t`, "\t",
		`\r`, "\r",
		`\"`, `"`,
		`\`, `\`,
	)
	return replacer.Replace(s)
}
