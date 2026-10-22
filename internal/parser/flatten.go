package parser

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/hellpuffyt/configsync/internal/model"
)

// Flatten walks an arbitrary decoded tree (as produced by encoding/json,
// gopkg.in/yaml.v3, or the internal TOML parser) and writes flattened
// dotted-path entries into out. Maps become "parent.child" paths, slices
// become "parent.0", "parent.1", and so on.
func Flatten(v interface{}, prefix string, out model.Config) {
	switch val := v.(type) {
	case map[string]interface{}:
		if len(val) == 0 && prefix != "" {
			out[prefix] = model.Value{Raw: "", Kind: model.KindNull}
			return
		}
		for k, sub := range val {
			key := joinPath(prefix, k)
			Flatten(sub, key, out)
		}
	case map[interface{}]interface{}:
		// yaml.v2-style maps, kept for robustness.
		for k, sub := range val {
			key := joinPath(prefix, fmt.Sprintf("%v", k))
			Flatten(sub, key, out)
		}
	case []interface{}:
		if len(val) == 0 && prefix != "" {
			out[prefix] = model.Value{Raw: "", Kind: model.KindNull}
			return
		}
		for i, sub := range val {
			key := joinPath(prefix, strconv.Itoa(i))
			Flatten(sub, key, out)
		}
	case nil:
		if prefix != "" {
			out[prefix] = model.Value{Raw: "", Kind: model.KindNull}
		}
	case bool:
		if prefix != "" {
			out[prefix] = model.Value{Raw: strconv.FormatBool(val), Kind: model.KindBool}
		}
	case string:
		if prefix != "" {
			out[prefix] = model.Value{Raw: val, Kind: model.KindString}
		}
	case int:
		if prefix != "" {
			out[prefix] = model.Value{Raw: strconv.Itoa(val), Kind: model.KindNumber}
		}
	case int64:
		if prefix != "" {
			out[prefix] = model.Value{Raw: strconv.FormatInt(val, 10), Kind: model.KindNumber}
		}
	case uint64:
		if prefix != "" {
			out[prefix] = model.Value{Raw: strconv.FormatUint(val, 10), Kind: model.KindNumber}
		}
	case float64:
		if prefix != "" {
			out[prefix] = model.Value{Raw: formatFloat(val), Kind: model.KindNumber}
		}
	case json.Number:
		if prefix != "" {
			out[prefix] = model.Value{Raw: formatJSONNumber(val), Kind: model.KindNumber}
		}
	default:
		if prefix != "" {
			out[prefix] = model.Value{Raw: fmt.Sprintf("%v", val), Kind: model.KindString}
		}
	}
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// formatFloat renders a float64 without a trailing ".0" when it represents
// a whole number, so JSON's "8080" and YAML's "8080" compare equal instead
// of showing up as a spurious value-differs.
func formatFloat(f float64) string {
	// Only convert when the value is integral and well inside int64 range;
	// converting a larger float to int64 is undefined.
	if f == math.Trunc(f) && math.Abs(f) < 1e18 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// formatJSONNumber renders a JSON number exactly when it is an integer
// (of any size), and like formatFloat otherwise, so "8080" and "8080.0"
// still match YAML's 8080.
func formatJSONNumber(n json.Number) string {
	s := n.String()
	if i, err := n.Int64(); err == nil {
		return strconv.FormatInt(i, 10)
	}
	if isDigits(strings.TrimPrefix(s, "-")) {
		return s // integer too large for int64: keep every digit
	}
	if f, err := n.Float64(); err == nil {
		return formatFloat(f)
	}
	return s
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
