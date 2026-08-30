package parser

import (
	"fmt"
	"strconv"

	"github.com/prabeshsharma/configsync/internal/model"
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
	case float64:
		if prefix != "" {
			out[prefix] = model.Value{Raw: formatFloat(val), Kind: model.KindNumber}
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
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}
