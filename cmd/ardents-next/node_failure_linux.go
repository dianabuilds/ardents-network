//go:build linux

package main

import (
	"encoding/json"

	"github.com/dianabuilds/ardents-network/internal/successor/node"
)

// Preserve the original one-object/string-field role report. The encoded local
// fact contains no raw error text or private role binding and grants no authority.
func encodeNodeFailureDiagnostic(fact *node.FailureDiagnostic) string {
	if fact == nil {
		return ""
	}
	raw, err := json.Marshal(fact)
	if err != nil {
		return `{"cause":"diagnostic-unavailable"}`
	}
	return string(raw)
}
