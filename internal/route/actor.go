package route

import (
	"context"
	"crypto/tls"
	"errors"
	"time"
)

// Run owns one bounded Client, Node-position, or Publisher process lifetime.
func Run(ctx context.Context, input Actor, ready func(Evidence)) (Evidence, error) {
	if input.ManifestDigest == [32]byte{} {
		return Evidence{}, errors.New("route manifest commitment is required")
	}
	if err := validateDeadline(input.Deadline); err != nil {
		return Evidence{}, err
	}
	attempt, cancel := context.WithTimeout(ctx, input.Deadline)
	defer cancel()
	var result Evidence
	var err error
	switch input.Role {
	case "client":
		if ready != nil {
			return Evidence{}, errors.New("client has no listener readiness event")
		}
		result, err = transfer(attempt, input)
	case "publisher":
		result, err = servePublisher(attempt, input, ready)
	case "initiator", "introduction", "rendezvous", "responder":
		result, err = serveNode(attempt, input, ready)
	default:
		return Evidence{}, errors.New("route actor role is invalid")
	}
	result.ManifestDigest = input.ManifestDigest
	result.DeadlineMillis = uint32(input.Deadline / time.Millisecond)
	result.Cleanup = true
	result.Terminal = "success"
	if err != nil {
		result.Terminal = "error"
		result.Error = err.Error()
		result.Cancelled = attempt.Err() != nil || ctx.Err() != nil
	}
	result.SourceID, result.BuildDigest, err = buildIdentity(result.SourceID, result.BuildDigest, err)
	if err != nil && result.Terminal == "success" {
		result.Terminal = "error"
		result.Error = err.Error()
	}
	return result, err
}

func emptyPlan(value Plan) bool {
	return value.NetworkID == [32]byte{} && value.Generation == "" && value.Epoch == 0 &&
		value.Digest == [32]byte{} && value.Profile == "" && value.ViewRoot == [32]byte{} && value.Seed == [32]byte{} && value.SelectionAt == 0 &&
		len(value.ExcludedIdentities) == 0 && len(value.ExcludedFamilies) == 0 && len(value.ExcludedDomains) == 0 &&
		len(value.Positions) == 0
}

func emptyCertificate(value tls.Certificate) bool {
	return len(value.Certificate) == 0 && value.PrivateKey == nil
}
