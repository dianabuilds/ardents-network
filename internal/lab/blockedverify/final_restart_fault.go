package blockedverify

import (
	"bytes"
	"encoding/json"
	"io"
)

type finalG4Phase struct {
	Regime    bool     `json:"regime"`
	Attempt   bool     `json:"attempt"`
	AttemptID [32]byte `json:"attempt_id"`
	Deadline  int64    `json:"deadline"`
	Contacts  int      `json:"contacts"`
	Terminal  string   `json:"terminal"`
}

type finalG4Reopen struct {
	Kind     string `json:"kind"`
	Phase    string `json:"phase"`
	Terminal string `json:"terminal"`
	Attempt  bool   `json:"attempt"`
	Contacts int    `json:"contacts"`
}

type finalG4Receipt struct {
	Schema      string        `json:"schema"`
	Phase       string        `json:"phase"`
	Checkpoint  finalG4Phase  `json:"checkpoint"`
	Reopened    finalG4Reopen `json:"reopened"`
	AtomicWith  string        `json:"atomic_with"`
	Observation string        `json:"observation"`
}

func validFinalG4Receipt(raw []byte, variant string) bool {
	var value finalG4Receipt
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		value.Schema != "ardents-h3-g4-receipt-v1" || value.Phase != variant ||
		value.Reopened.Kind != "g4-reopen" || value.Reopened.Phase != variant {
		return false
	}
	contacts, observation := 1, "durable-generation"
	terminal, attempt := "bridge-interrupted", true
	switch variant {
	case "after-import":
		contacts, observation, terminal, attempt = 0, "imported-state", "", false
	case "after-exposure-1":
		contacts = 2
	case "after-exposure-2":
		contacts = 3
	case "after-exposure-3":
		contacts = 4
	case "after-adapter-start":
		observation = "candidate-state-root"
	case "after-readiness":
		observation = "endpoint-ready-evidence"
	case "after-useful-work-prefix":
		observation = "publisher-progress"
	case "after-terminal-record":
		observation, terminal = "terminal-generation", "opened"
	case "during-cleanup":
		observation = "cleanup-callback"
	case "after-regime-publication", "after-exposure-0":
	default:
		return false
	}
	atomic := value.AtomicWith == ""
	if variant == "after-regime-publication" {
		atomic = value.AtomicWith == "after-exposure-0"
	}
	checkpointTerminal := ""
	if variant == "after-terminal-record" {
		checkpointTerminal = "opened"
	}
	return atomic && value.Observation == observation && value.Checkpoint.Regime == attempt &&
		value.Checkpoint.Attempt == attempt && value.Checkpoint.Contacts == contacts &&
		value.Checkpoint.Terminal == checkpointTerminal && value.Reopened.Attempt == attempt &&
		value.Reopened.Contacts == contacts && value.Reopened.Terminal == terminal
}
