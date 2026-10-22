package parser

import (
	"bytes"
	"encoding/json"

	"github.com/hellpuffyt/configsync/internal/model"
)

// ParseJSON parses a JSON document and flattens it to dotted paths.
func ParseJSON(data []byte) (model.Config, error) {
	// UseNumber keeps numbers as written: decoding to float64 rounded
	// integers above 2^53, so two different large IDs compared equal.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var tree interface{}
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	out := make(model.Config)
	Flatten(tree, "", out)
	return out, nil
}
