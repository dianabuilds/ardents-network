// Package trust owns trust anchors and peer trust evaluation.
// It does not own identity authentication or policy enforcement.
package trust

import (
	"sort"
	"time"

	discoveryrecord "ardents/internal/discovery/records"
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
	now    func() time.Time
}

func NewEvaluator() *Evaluator {
	return &Evaluator{
		state:  "new",
		anchor: map[string]struct{}{},
		now:    func() time.Time { return time.Now().UTC() },
	}
}

func (s *Evaluator) Trust(key string) {
	if key == "" {
		return
	}
	s.anchor[key] = struct{}{}
}

func (s *Evaluator) Evaluate(record discoveryrecord.Record) Result {
	return EvaluateRecordAt(s.anchor, record, s.now())
}

func (s *Evaluator) Remember(result Result) {
	s.last = result
	s.state = stateForResult(result)
}

func EvaluateRecord(anchor map[string]struct{}, record discoveryrecord.Record) Result {
	return EvaluateRecordAt(anchor, record, time.Now().UTC())
}

func EvaluateRecordAt(anchor map[string]struct{}, record discoveryrecord.Record, now time.Time) Result {
	result := Result{
		Outcome: "unverified",
		Reason:  "record verification not attempted",
	}
	if err := discoveryrecord.ValidateRetained(record); err != nil {
		result.Reason = err.Error()
		return result
	}
	if now.Before(record.IssuedAt) {
		result.Outcome = "not_yet_valid"
		result.Reason = "record is not yet valid"
		return result
	}
	if !record.ExpiresAt.After(now) {
		result.Outcome = "expired"
		result.Reason = "record expired"
		return result
	}

	result.Valid = true
	result.Outcome = "verified"
	result.Reason = "record verified"
	if _, ok := anchor[record.PublicKeyText()]; ok {
		result.Trusted = true
		result.Usable = true
		result.Outcome = "usable"
		result.Reason = "record verified and trusted"
	}
	return result
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
