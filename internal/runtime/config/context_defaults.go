package config

import (
	"encoding/json"
	"path/filepath"
)

func applyContextDefaults(raw []byte, doc *Document) {
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil {
		return
	}
	if !nestedFieldPresent(root, "node", "data_dir") {
		doc.Node.DataDir = filepath.Join("var", doc.Node.Name)
	}
	if !nestedFieldPresent(root, "network", "store_path") {
		doc.Network.StorePath = filepath.Join(doc.Node.DataDir, "waku-store.db")
	}
}

func nestedFieldPresent(root map[string]json.RawMessage, section, field string) bool {
	raw, ok := root[section]
	if !ok {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return false
	}
	_, ok = fields[field]
	return ok
}
