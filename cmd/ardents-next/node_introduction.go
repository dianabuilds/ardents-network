package main

import (
	"encoding/json"
	"io"
)

func nodeIntroductionReport(out io.Writer, phase, outcome, first, cleanup string) error {
	return json.NewEncoder(out).Encode(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
		First     string `json:"first_owner,omitempty"`
		Cleanup   string `json:"cleanup_owner,omitempty"`
	}{"node.introduction", phase, outcome, first, cleanup})
}
