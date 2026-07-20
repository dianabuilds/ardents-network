package discovery

import discoverytrust "ardents/internal/discovery/trust"

type TrustResult = discoverytrust.Result

type TrustEvaluator = discoverytrust.Evaluator

func NewTrustEvaluator() *TrustEvaluator {
	return discoverytrust.NewEvaluator()
}

func EvaluateTrustRecord(anchor map[string]struct{}, record Record) TrustResult {
	return discoverytrust.EvaluateRecord(anchor, record)
}
