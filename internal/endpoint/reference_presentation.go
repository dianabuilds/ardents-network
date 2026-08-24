package endpoint

import "github.com/dianabuilds/ardents-network/internal/endpoint/reference"

// ReferencePresentation is the browser-compatible presentation input for one
// Target already authenticated by Endpoint. It has no Link parser, resolution,
// route selection, or Service authority.
type ReferencePresentation struct {
	AuthenticatedTarget [32]byte
	Document            reference.Resource
	Resources           map[string]reference.Resource
}

// OpenReferencePresentation creates one scoped local presentation origin for
// an already-authenticated Target. Its caller owns the live Service Connection
// and must close the returned server when that connection ends.
func OpenReferencePresentation(input ReferencePresentation) (*reference.Server, error) {
	return reference.Open(reference.Config{Target: input.AuthenticatedTarget, Document: input.Document, Resources: input.Resources})
}
