package infrastructure

import (
	"bytes"
	"encoding/json"
)

// This is immutable configuration with explicit predecessor provenance, never
// an Authorization or a join claim. Root independently consumes the complete
// initial/replacement/start/retirement transcripts before it can mutate or Start.
// Non-root Startup still performs its own original native/peer admission.
type sourceReplacementBinding struct {
	Schema                   string        `json:"schema"`
	Configuration            sourceBinding `json:"configuration"`
	InitialGeneration        string        `json:"initial_generation"`
	PredecessorBindingSHA256 string        `json:"predecessor_binding_sha256"`
	RetirementSHA256         string        `json:"retirement_sha256"`
	PredecessorAttempt       uint32        `json:"predecessor_attempt"`
}

func decodeSourceReplacementBinding(raw []byte) (sourceReplacementBinding, error) {
	var r sourceReplacementBinding
	if len(raw) == 0 || len(raw) > 32<<10 || json.Unmarshal(raw, &r) != nil {
		return sourceReplacementBinding{}, ErrInput
	}
	canonical, err := json.Marshal(r)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return sourceReplacementBinding{}, ErrInput
	}
	b := r.Configuration
	if r.Schema != "ardents-source-replacement-binding-v1" || b.Schema != "ardents-source-replacement-configuration-v1" ||
		!sourceInitialSHA(b.Predecessor) || b.Predecessor == b.Generation || !sourceInitialSHA(r.InitialGeneration) ||
		!sourceInitialSHA(r.PredecessorBindingSHA256) || !sourceInitialSHA(r.RetirementSHA256) || r.PredecessorAttempt >= uint32(sourceAttemptLimit)-1 {
		return sourceReplacementBinding{}, ErrBinding
	}
	if err := validateSourceBindingConfiguration(b); err != nil {
		return sourceReplacementBinding{}, err
	}
	return r, nil
}

// Only an exact initial or distinct replacement grammar can describe selected
// configuration. This projection grants neither original retirement nor lineage
// acceptance: the root transition reader must authenticate those separately.
func decodeSourceSelectedBinding(raw []byte) (sourceBinding, *sourceReplacementBinding, error) {
	if b, err := decodeSourceBinding(raw); err == nil {
		return b, nil, nil
	}
	r, err := decodeSourceReplacementBinding(raw)
	if err != nil {
		return sourceBinding{}, nil, err
	}
	return r.Configuration, &r, nil
}
