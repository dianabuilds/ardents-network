package delivery

import (
	"context"

	requestinput "github.com/dianabuilds/ardents-network/internal/successor/endpoint/delivery/request"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/release/endpoint"
)

// ProvisionResult reports the Installation outcome, not qualified Execution,
// accepting Service readiness or a reusable Release authorization. A successor
// result may retain an irreversible full ACK together with a cleanup error.
type ProvisionResult struct {
	Status           string
	GenerationDigest string
}

// ReadProvisionRequest observes native initial prerequisites before command
// composition opens Release history. Its result is still not installation
// authority; ProvisionInitial repeats admission before effects.
func ReadProvisionRequest(ctx context.Context, filename string) (requestinput.Document, error) {
	return readProvisionRequest(ctx, filename)
}

// ProvisionInitial consumes genuine fresh initial authorization and original
// native request custody. Composition retains the separate Release verifier
// through this operation and its physical cleanup. No implicit start occurs.
func ProvisionInitial(ctx context.Context, request requestinput.Document, authorization generationauthorization.Authorization) (ProvisionResult, error) {
	return provisionInitial(ctx, request, authorization)
}
