package forwarding

import (
	"bytes"
	"encoding/json"
)

// replacementBinding preserves the initial provenance and one exact predecessor.
// These copied declarations supply no Release, retirement or recovery evidence.
type replacementBinding struct {
	Schema                   string  `json:"schema"`
	Configuration            binding `json:"configuration"`
	InitialGeneration        string  `json:"initial_generation"`
	PredecessorGeneration    string  `json:"predecessor_generation"`
	PredecessorBindingSHA256 string  `json:"predecessor_binding_sha256"`
	RetirementSHA256         string  `json:"retirement_sha256"`
	PredecessorAttempt       uint32  `json:"predecessor_attempt"`
}

// generationSelection is byte provenance only. Initial and replacement formats
// remain distinct; every consumer obtains its own native/authority admission.
type generationSelection struct {
	binding     binding
	replacement *replacementBinding
	raw         []byte
}

func decodeSelection(raw []byte) (generationSelection, error) {
	if len(raw) == 0 || len(raw) > 32<<10 {
		return generationSelection{}, ErrInput
	}
	var header struct {
		Schema string `json:"schema"`
	}
	if json.Unmarshal(raw, &header) != nil {
		return generationSelection{}, ErrInput
	}
	if header.Schema == "ardents-forwarding-binding-v1" {
		b, err := decodeBinding(raw)
		if err != nil {
			return generationSelection{}, err
		}
		return generationSelection{binding: b, raw: bytes.Clone(raw)}, nil
	}
	if header.Schema != "ardents-forwarding-replacement-binding-v1" {
		return generationSelection{}, ErrBinding
	}
	var r replacementBinding
	if json.Unmarshal(raw, &r) != nil {
		return generationSelection{}, ErrInput
	}
	canonical, err := json.Marshal(r)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return generationSelection{}, ErrBinding
	}
	configuration, err := json.Marshal(r.Configuration)
	if err != nil {
		return generationSelection{}, err
	}
	b, err := decodeConfiguration(append(configuration, '\n'), "ardents-forwarding-replacement-configuration-v1")
	if err != nil || !initialSHA(r.InitialGeneration) || !initialSHA(r.PredecessorGeneration) || !initialSHA(r.PredecessorBindingSHA256) || !initialSHA(r.RetirementSHA256) || r.PredecessorAttempt >= attemptLimit-1 || b.Generation == r.InitialGeneration || b.Generation == r.PredecessorGeneration {
		return generationSelection{}, ErrBinding
	}
	return generationSelection{binding: b, replacement: &r, raw: bytes.Clone(raw)}, nil
}

func (s generationSelection) initialGeneration() string {
	if s.replacement != nil {
		return s.replacement.InitialGeneration
	}
	return s.binding.Generation
}

// matchPredecessor verifies declarations, not the original retirement, forward
// Release version or native history. The lifecycle owner must prove those.
func (s generationSelection) matchPredecessor(previous generationSelection) error {
	r := s.replacement
	p, b := previous.binding, s.binding
	if r == nil || len(previous.raw) == 0 || r.PredecessorGeneration != p.Generation || r.PredecessorBindingSHA256 != digest(previous.raw) || r.InitialGeneration != previous.initialGeneration() || !sameRetainedRequest(b.Request, p.Request) || b.Account != p.Account || b.UID != p.UID || b.GID != p.GID || b.History != p.History || b.CertificateSHA256 != p.CertificateSHA256 || b.TemplateSHA256 != p.TemplateSHA256 {
		return ErrBinding
	}
	if previous.replacement != nil && r.PredecessorAttempt <= previous.replacement.PredecessorAttempt {
		return ErrBinding
	}
	return nil
}

func sameRetainedRequest(a, b Request) bool {
	// Delivery source files may be freshly prepared at new protected paths.
	// Their bytes are checked separately; installed credentials and roots stay.
	a.Plan, a.Certificate, a.PrivateKey = b.Plan, b.Certificate, b.PrivateKey
	return a == b
}
