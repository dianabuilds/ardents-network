// Package trust owns trust anchors and peer trust evaluation.
// It does not own identity authentication or policy enforcement.
package trust

import (
	"crypto/ed25519"
	"encoding/base64"
	"sort"
	"time"

	discoveryrecord "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
)

type Result struct {
	Outcome string
	Valid   bool
	Trusted bool
	Usable  bool
	Reason  string
}

type Evaluator struct {
	state  string
	last   Result
	anchor map[string]struct{}
}

func NewEvaluator() *Evaluator {
	return &Evaluator{
		state:  "new",
		anchor: map[string]struct{}{},
	}
}

func (s *Evaluator) Trust(key string) {
	if key == "" {
		return
	}
	s.anchor[key] = struct{}{}
}

func (s *Evaluator) Evaluate(record discoveryrecord.Record) Result {
	return EvaluateRecord(s.anchor, record)
}

func (s *Evaluator) Remember(result Result) {
	s.last = result
	s.state = stateForResult(result)
}

func EvaluateRecord(anchor map[string]struct{}, record discoveryrecord.Record) Result {
	result := Result{
		Outcome: "unverified",
		Reason:  "record verification not attempted",
	}

	if rejected, ok := rejectedRecord(record, result); ok {
		return rejected
	}
	if rejected, ok := rejectedIdentityBinding(record, result); ok {
		return rejected
	}

	publicKey, signature, payload, rejected, ok := verifiedSignatureInputs(record, result)
	if !ok {
		return rejected
	}

	result.Valid = ed25519.Verify(publicKey, payload, signature)
	if !result.Valid {
		result.Reason = "signature verification failed"
		return result
	}

	result.Outcome = "verified"
	result.Reason = "record verified"
	if _, ok := anchor[record.PublicKey]; ok {
		result.Trusted = true
		result.Usable = true
		result.Outcome = "usable"
		result.Reason = "record verified and trusted"
	}
	return result
}

func rejectedRecord(record discoveryrecord.Record, result Result) (Result, bool) {
	if record.ID == "" || record.Subject == "" || record.PublicKey == "" || record.Signature == "" {
		result.Reason = "record is incomplete"
		return result, true
	}
	if !record.ExpiresAt.IsZero() && time.Now().UTC().After(record.ExpiresAt) {
		result.Outcome = "expired"
		result.Reason = "record expired"
		return result, true
	}
	return Result{}, false
}

func rejectedIdentityBinding(record discoveryrecord.Record, result Result) (Result, bool) {
	expectedPrincipal, err := identityprincipal.FromPublicKey(record.PublicKey)
	if err != nil {
		result.Reason = err.Error()
		return result, true
	}
	if record.Node != expectedPrincipal {
		result.Reason = "record node does not match signing identity"
		return result, true
	}
	if record.Kind == "node" && (record.Subject != expectedPrincipal || record.ID != expectedPrincipal+":node") {
		result.Reason = "record principal does not match signing identity"
		return result, true
	}
	return Result{}, false
}

func verifiedSignatureInputs(record discoveryrecord.Record, result Result) (ed25519.PublicKey, []byte, []byte, Result, bool) {
	publicKey, err := base64.StdEncoding.DecodeString(record.PublicKey)
	if err != nil {
		result.Reason = "public key is invalid"
		return nil, nil, nil, result, false
	}
	signature, err := base64.StdEncoding.DecodeString(record.Signature)
	if err != nil {
		result.Reason = "signature is invalid"
		return nil, nil, nil, result, false
	}
	payload, err := discoveryrecord.Canonical(record)
	if err != nil {
		result.Reason = "payload encoding failed"
		return nil, nil, nil, result, false
	}
	return publicKey, signature, payload, Result{}, true
}

func (s *Evaluator) State() string {
	return s.state
}

func (s *Evaluator) Last() Result {
	return s.last
}

func (s *Evaluator) Anchors() []string {
	out := make([]string, 0, len(s.anchor))
	for key := range s.anchor {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func stateForResult(result Result) string {
	if result.Valid {
		return "ready"
	}
	if result.Outcome == "" && result.Reason == "" {
		return "new"
	}
	return "degraded"
}
