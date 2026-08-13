package main

import (
	"github.com/dianabuilds/ardents-network/internal/qualification/servicesmoke"
	statequalification "github.com/dianabuilds/ardents-network/internal/qualification/state"
)

func evaluateServiceSmoke(arguments []string) (statequalification.Result, error) {
	config, err := servicesmoke.ParseConfig(arguments)
	if err != nil {
		return statequalification.Result{}, err
	}
	result := servicesmoke.Run(config)
	return statequalification.Result{Verdict: result.Verdict, Reason: result.Reason, EvidenceRoot: result.EvidenceRoot, EvidenceDigest: result.EvidenceDigest}, nil
}
