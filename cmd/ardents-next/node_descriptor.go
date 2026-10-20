package main

import (
	"encoding/json"
	"io"
)

func nodeDescriptorReport(out io.Writer, phase, outcome, first, cleanup string, details ...string) error {
	firstFailure := ""
	if len(details) == 1 {
		firstFailure = details[0]
	}
	return json.NewEncoder(out).Encode(struct {
		FirstFailure string `json:"first_failure,omitempty"`
		Operation    string `json:"operation"`
		Phase        string `json:"phase"`
		Outcome      string `json:"outcome"`
		First        string `json:"first_owner,omitempty"`
		Cleanup      string `json:"cleanup_owner,omitempty"`
	}{firstFailure, "node.descriptor", phase, outcome, first, cleanup})
}
