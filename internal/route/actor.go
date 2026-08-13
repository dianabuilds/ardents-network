package route

import (
	"context"
	"crypto/tls"
	"errors"
)

// Run owns one bounded Client, Node-position, or Publisher process lifetime.
func Run(ctx context.Context, input Actor, ready func(Evidence)) (Evidence, error) {
	switch input.Role {
	case "client":
		if ready != nil {
			return Evidence{}, errors.New("client has no listener readiness event")
		}
		return transfer(ctx, input)
	case "publisher":
		return servePublisher(ctx, input, ready)
	case "initiator", "introduction", "rendezvous", "responder":
		return serveNode(ctx, input, ready)
	default:
		return Evidence{}, errors.New("route actor role is invalid")
	}
}

func emptyPlan(value Plan) bool {
	return value.NetworkID == [32]byte{} && value.Generation == "" && value.Epoch == 0 &&
		value.Digest == [32]byte{} && len(value.Positions) == 0
}

func emptyCertificate(value tls.Certificate) bool {
	return len(value.Certificate) == 0 && value.PrivateKey == nil
}
