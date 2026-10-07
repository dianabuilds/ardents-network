package installation

import "context"

// ProvisionResult reports one completed stopped installation, not qualified
// Execution, accepting Service readiness or a reusable Release authorization.
type ProvisionResult struct {
	Status           string
	GenerationDigest string
}

// ReadProvisionRequest observes native initial prerequisites before command
// composition opens Release history. Its result is still not installation
// authority; ProvisionInitial repeats admission before effects.
func ReadProvisionRequest(ctx context.Context, filename string) (Request, error) {
	return readProvisionRequest(ctx, filename)
}

// ProvisionInitial consumes genuine fresh initial authorization and original
// native request custody. Composition retains the separate Release verifier
// through this operation and its physical cleanup. No implicit start occurs.
func ProvisionInitial(ctx context.Context, request Request, authorization Authorization) (ProvisionResult, error) {
	return provisionInitial(ctx, request, authorization)
}
