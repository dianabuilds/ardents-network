//go:build browsercompat

package endpoint

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/endpoint/reference"
)

// ErrReferenceTargetMismatch reports a browser presentation whose authenticated
// connection Target is not the Target explicitly selected by the User's Link.
var ErrReferenceTargetMismatch = errors.New("authenticated reference Target does not match Target Link")

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

// OpenReferenceFromLink creates a presentation origin only when the User's
// exact Target Link names the Target that this Endpoint already authenticated.
// It neither resolves the Link nor establishes the Service Connection; those
// actions must complete before the caller supplies ReferencePresentation.
func (endpoint *endpoint) OpenReferenceFromLink(text string, input ReferencePresentation) (*reference.Server, error) {
	target, err := endpoint.TargetFromLink(text)
	if err != nil {
		return nil, err
	}
	if target != input.AuthenticatedTarget {
		return nil, ErrReferenceTargetMismatch
	}
	return OpenReferencePresentation(input)
}
