package main

import (
	"github.com/dianabuilds/ardents-network/internal/qualification/recoverysmoke"
	"github.com/dianabuilds/ardents-network/internal/qualification/servicesmoke"
	statequalification "github.com/dianabuilds/ardents-network/internal/qualification/state"
)

func evaluateDockerSmoke(kind string, arguments []string) (statequalification.Result, error) {
	if kind == "recovery" {
		result, err := recoverysmoke.Execute(arguments)
		return statequalification.Result{Verdict: result.Verdict, Reason: result.Reason,
			EvidenceRoot: result.EvidenceRoot, EvidenceDigest: result.EvidenceDigest}, err
	}
	result, err := servicesmoke.Execute(arguments)
	return statequalification.Result{Verdict: result.Verdict, Reason: result.Reason,
		EvidenceRoot: result.EvidenceRoot, EvidenceDigest: result.EvidenceDigest}, err
}
