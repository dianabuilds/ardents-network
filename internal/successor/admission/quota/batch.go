package quota

import "github.com/dianabuilds/ardents-network/internal/successor/admission"

import (
	"bytes"
	"context"
	"crypto/sha256"
)

type verifiedBatch struct {
	request, digest, permission, commitment [32]byte
	window                                  uint64
	class                                   uint8
	count                                   uint16
	maxima                                  [3]uint32
}

func verifyBatch(ctx context.Context, raw []byte, facts admission.Facts, binding LedgerBinding) (verifiedBatch, admission.Outcome) {
	var b verifiedBatch
	request, outcome := admission.InspectClosedTokenBatch(ctx, raw, facts)
	if outcome != admission.Accepted {
		return b, outcome
	}
	if !binding.matches(facts) {
		return b, admission.Binding
	}
	b.request = request.RequestID
	b.permission = request.Permission.PermissionID
	b.window = uint64(request.WindowStart.Unix())
	b.class = request.Class
	b.count = uint16(len(request.BlindedRequests))
	spki := request.SPKI[:]
	found := false
	for _, key := range binding.Keys {
		if key.Window == b.window && key.Class == b.class && bytes.Equal(key.SPKI, spki) {
			found = true
			break
		}
	}
	if !found {
		return b, admission.Binding
	}
	if ctx.Err() != nil {
		return b, admission.Canceled
	}
	b.digest = sha256.Sum256(raw)
	permission, _ := admission.EncodePermission(request.Permission)
	b.commitment = sha256.Sum256(permission)
	b.maxima = request.Permission.Maxima
	return b, admission.Accepted
}
