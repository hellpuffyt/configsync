package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hellpuffyt/configsync/internal/model"
)

// ParseProperties parses a Java-style .properties file: "key=value" or
// "key: value" or "key value" pairs, '#'/'!' comments, backslash line
// continuations, and basic \: \= \ \n \t escapes.
func ParseProperties(data []byte) (model.Config, error) {
	out := make(model.Config)

	rawLines := strings.Split(string(data), "\n")
	logical := joinContinuations(rawLines)

	for _, line := range logical {
		trimmed := strings.TrimLeft(line, " \t\f")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			continue
		}
		key, value, err := splitPropertyLine(trimmed)
		if err != nil {
			return nil, err
		}
		out[key] = model.Value{Raw: value, Kind: model.KindString}
	}
	return out, nil
}

// joinContinuations merges lines ending in an odd number of trailing
// backslashes with the following line, per the .properties spec.
func joinContinuations(lines []string) []string {
	var out []string
	var buf strings.Builder
	joining := false
	for _, l := range lines {
		l = strings.TrimRight(l, "\r")
		if joining {
			buf.WriteString(strings.TrimLeft(l, " \t\f"))
		} else {
			buf.Reset()
			buf.WriteString(l)
		}
		if endsWithOddBackslashes(l) {
			// Strip the trailing backslash and keep accumulating.
			s := buf.String()
			buf.Reset()
			buf.WriteString(strings.TrimSuffix(s, "\\"))
			joining = true
			continue
		}
		out = append(out, buf.String())
		joining = false
	}
	if joining {
		out = append(out, buf.String())
	}
	return out
}

func endsWithOddBackslashes(s string) bool {
	n := 0
	for i := len(s) - 1; i >= 0 && s[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

func splitPropertyLine(line string) (key, value string, err error) {
	// Find the first unescaped separator: '=', ':', or whitespace.
	i := 0
	var keyBuf strings.Builder
	for i < len(line) {
		c := line[i]
		if c == '\\' && i+1 < len(line) {
			text, n := propertyEscape(line[i:])
			keyBuf.WriteString(text)
			i += n
			continue
		}
		if c == '=' || c == ':' || c == ' ' || c == '\t' {
			break
		}
		keyBuf.WriteByte(c)
		i++
	}
	key = keyBuf.String()
	if key == "" {
		return "", "", fmt.Errorf("empty property key in line %q", line)
	}
	// Skip whitespace, then one optional '=' or ':', then whitespace.
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i < len(line) && (line[i] == '=' || line[i] == ':') {
		i++
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
	}
	value = unescapeProperty(line[i:])
	return key, value, nil
}

func unescapeChar(c byte) byte {
	switch c {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	default:
		return c
	}
}

func unescapeProperty(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\\' && i+1 < len(s) {
			text, n := propertyEscape(s[i:])
			b.WriteString(text)
			i += n
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// propertyEscape decodes the escape at the start of s (which begins with a
// backslash) and returns the text and the number of bytes consumed. It
// handles \uXXXX, which Java .properties files use for non-ASCII text.
func propertyEscape(s string) (string, int) {
	if s[1] == 'u' && len(s) >= 6 {
		if r, err := strconv.ParseUint(s[2:6], 16, 32); err == nil {
			return string(rune(r)), 6
		}
	}
	return string(unescapeChar(s[1])), 2
}
