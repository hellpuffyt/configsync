package parser

import (
	"github.com/prabeshsharma/configsync/internal/model"
	"gopkg.in/yaml.v3"
)

// ParseYAML parses a YAML document and flattens it to dotted paths.
func ParseYAML(data []byte) (model.Config, error) {
	var tree interface{}
	if err := yaml.Unmarshal(data, &tree); err != nil {
		return nil, err
	}
	tree = normalizeYAML(tree)
	out := make(model.Config)
	Flatten(tree, "", out)
	return out, nil
}

// normalizeYAML converts yaml.v3's map[string]interface{} nodes (and any
// nested int/uint variants) into the plain types Flatten understands.
func normalizeYAML(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, sub := range val {
			out[k] = normalizeYAML(sub)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, sub := range val {
			out[i] = normalizeYAML(sub)
		}
		return out
	case int:
		return int64(val)
	case uint64:
		return int64(val)
	default:
		return val
	}
}
