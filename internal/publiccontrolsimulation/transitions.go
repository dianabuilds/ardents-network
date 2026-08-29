package publiccontrolsimulation

import (
	"errors"
	"strings"
	"time"
)

const (
	transitionReportSchema    = "ardents-h4-6d-transition-simulation-v1"
	transitionContractVersion = "h4-6d-project-control-transitions-v1"
)

// TransitionReport records one bounded H4-6D project-control transition run.
// It cannot authorize an Endpoint or qualify a public operation.
type TransitionReport struct {
	Schema                 string           `json:"schema"`
	Contract               string           `json:"contract"`
	SimulationResult       string           `json:"simulation_result"`
	DeclaredSourceRevision string           `json:"declared_source_revision"`
	ReceiptDigest          string           `json:"receipt_digest"`
	Simulation             bool             `json:"simulation"`
	Qualified              bool             `json:"qualified"`
	Passed                 []TransitionCell `json:"passed"`
	Rejected               []string         `json:"rejected"`
	Limitation             string           `json:"limitation"`
}

// TransitionCell binds one named matrix case to the exact evaluator outcome.
type TransitionCell struct {
	Case    string `json:"case"`
	Outcome string `json:"outcome"`
}

type transitionInput struct {
	FloorGeneration     uint64
	CandidateGeneration uint64
	Overlap             bool
	At                  time.Time
	NotAfter            time.Time
	Revoked             bool
	Compatible          bool
	Distributed         bool
	Emergency           bool
	EmergencyScope      string
	EmergencyExpiry     time.Time
}

type transitionDecision string

const (
	transitionOverlapAccepted transitionDecision = "overlap-accepted"
	transitionExpired         transitionDecision = "stop-expired"
	transitionRevoked         transitionDecision = "stop-revoked"
	transitionIncompatible    transitionDecision = "stop-incompatible-generation"
	transitionRollback        transitionDecision = "stop-rollback"
	transitionUnavailable     transitionDecision = "unavailable-distribution"
	transitionEmergency       transitionDecision = "stop-emergency-disabled"
	transitionInvalid         transitionDecision = "stop-invalid-transition"
)

// RunControlledTransitionsWithSourceRevision exercises the complete H4-6D
// transition matrix and emits a versioned, non-authorizing receipt.
func RunControlledTransitionsWithSourceRevision(sourceRevision string) (TransitionReport, error) {
	if sourceRevision == "" {
		return TransitionReport{}, errors.New("transition simulation source revision is required")
	}
	report := TransitionReport{Schema: transitionReportSchema, Contract: transitionContractVersion, DeclaredSourceRevision: sourceRevision, Simulation: true,
		Limitation: "project-controlled simulation; no Endpoint authority, independent operation, or Public Beta qualification"}
	at := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	live := func() transitionInput {
		return transitionInput{FloorGeneration: 1, CandidateGeneration: 2, Overlap: true, At: at, NotAfter: at.Add(time.Hour), Compatible: true, Distributed: true}
	}
	cases := []struct {
		pass     string
		expected transitionDecision
		input    transitionInput
	}{
		{"overlap-accepted", transitionOverlapAccepted, live()},
		{"expiry-stops", transitionExpired, func() transitionInput { value := live(); value.At = value.NotAfter; return value }()},
		{"revocation-stops", transitionRevoked, func() transitionInput { value := live(); value.Revoked = true; return value }()},
		{"incompatible-generation-stops", transitionIncompatible, func() transitionInput { value := live(); value.Compatible = false; return value }()},
		{"rollback-stops", transitionRollback, func() transitionInput {
			value := live()
			value.FloorGeneration = 2
			value.CandidateGeneration = 1
			return value
		}()},
		{"distribution-outage-stops", transitionUnavailable, func() transitionInput { value := live(); value.Distributed = false; return value }()},
		{"emergency-disablement-stops", transitionEmergency, func() transitionInput {
			value := live()
			value.Emergency = true
			value.EmergencyScope = "disable-only"
			value.EmergencyExpiry = at.Add(10 * time.Minute)
			return value
		}()},
	}
	for _, value := range cases {
		if decision := evaluateTransition(value.input); decision != value.expected {
			return TransitionReport{}, errors.New("transition simulation did not fail closed")
		}
		report.Passed = append(report.Passed, TransitionCell{Case: value.pass, Outcome: string(value.expected)})
	}
	for _, value := range []transitionInput{
		func() transitionInput { value := live(); value.Overlap = false; return value }(),
		func() transitionInput {
			value := live()
			value.Emergency = true
			value.EmergencyScope = "install-code"
			value.EmergencyExpiry = at.Add(time.Minute)
			return value
		}(),
		func() transitionInput {
			value := live()
			value.Emergency = true
			value.EmergencyScope = "disable-only"
			value.EmergencyExpiry = at
			return value
		}(),
	} {
		if evaluateTransition(value) != transitionInvalid {
			return TransitionReport{}, errors.New("transition simulation accepted an invalid transition")
		}
	}
	report.Rejected = []string{"overlap-without-continuity", "emergency-escalation", "emergency-expired"}
	report.SimulationResult = "passed"
	passed := make([]string, 0, len(report.Passed))
	for _, cell := range report.Passed {
		passed = append(passed, cell.Case+":"+cell.Outcome)
	}
	report.ReceiptDigest = digest([]byte(strings.Join([]string{report.Schema, report.Contract, report.SimulationResult, report.DeclaredSourceRevision,
		strings.Join(passed, ","), strings.Join(report.Rejected, ","), report.Limitation}, "\n")))
	return report, nil
}

func evaluateTransition(value transitionInput) transitionDecision {
	if !value.Distributed {
		return transitionUnavailable
	}
	if value.CandidateGeneration < value.FloorGeneration {
		return transitionRollback
	}
	if !value.Compatible {
		return transitionIncompatible
	}
	if value.Revoked {
		return transitionRevoked
	}
	if !value.At.Before(value.NotAfter) {
		return transitionExpired
	}
	if value.Emergency {
		if value.EmergencyScope != "disable-only" || !value.At.Before(value.EmergencyExpiry) {
			return transitionInvalid
		}
		return transitionEmergency
	}
	if value.Overlap && value.CandidateGeneration == value.FloorGeneration+1 {
		return transitionOverlapAccepted
	}
	return transitionInvalid
}
