package parser

import (
	"encoding/json"

	"github.com/prabeshsharma/configsync/internal/model"
)

// ParseJSON parses a JSON document and flattens it to dotted paths.
func ParseJSON(data []byte) (model.Config, error) {
	var tree interface{}
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, err
	}
	out := make(model.Config)
	Flatten(tree, "", out)
	return out, nil
}
