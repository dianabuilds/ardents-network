package main

import (
	"context"

	routesmoke "github.com/dianabuilds/ardents-network/internal/qualification/routesmoke"
	statequalification "github.com/dianabuilds/ardents-network/internal/qualification/state"
)

func evaluateRouteSmoke(arguments []string) (statequalification.Result, error) {
	config, err := routesmoke.ParseConfig(arguments)
	if err != nil {
		return statequalification.Result{}, err
	}
	result := routesmoke.Run(context.Background(), config)
	return statequalification.Result{Verdict: result.Verdict, Reason: result.Reason, EvidenceRoot: result.EvidenceRoot,
		EvidenceDigest: result.EvidenceDigest}, nil
}
