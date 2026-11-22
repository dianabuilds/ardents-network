package main

import (
	"encoding/json"
	"io"
)

func writeNodeEvent(out io.Writer, operation, phase, outcome, first, cleanup, failure string) error {
	return json.NewEncoder(out).Encode(struct {
		FirstFailure string `json:"first_failure,omitempty"`
		Operation    string `json:"operation"`
		Phase        string `json:"phase"`
		Outcome      string `json:"outcome"`
		First        string `json:"first_owner,omitempty"`
		Cleanup      string `json:"cleanup_owner,omitempty"`
	}{failure, operation, phase, outcome, first, cleanup})
}
