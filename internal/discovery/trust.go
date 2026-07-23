package discovery

import discoverytrust "ardents/internal/discovery/trust"
import identitytrust "ardents/internal/identity/trust"

type TrustResult = discoverytrust.Result

type TrustEvaluator = discoverytrust.Evaluator

func NewTrustEvaluator(registry *identitytrust.Registry) *TrustEvaluator {
	return discoverytrust.NewEvaluator(registry)
}
