package instance

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/publication"
)

const responseDomain = "ardents-service-instance-response-v3\x00"

var (
	ErrInvalid           = publication.ErrRequestInvalid
	ErrUnavailable       = publication.ErrResponseMismatch
	ErrPending           = errors.New("service Instance response pending")
	ErrSuccessorRequired = errors.New("service Instance successor required")
)

// RequestView is copied public approval input. Publication owns its grammar.
type RequestView = publication.RequestView

// ParseRequest retains the public Instance consumer's source contract. It grants
// no private material or live Instance authority.
func ParseRequest(raw []byte) (RequestView, error) {
	return publication.ParseServiceRequest(raw)
}

// InitializeRequest prepares the Instance and returns its public request after
// closing the original root.
func InitializeRequest(ctx context.Context, config Config) ([]byte, error) {
	root, err := Initialize(ctx, config)
	if err != nil {
		return nil, err
	}
	return requestAndClose(ctx, root)
}

// ReadRequest reopens a prepared Instance and returns the same public request
// after closing the original root.
func ReadRequest(ctx context.Context, path string) ([]byte, error) {
	root, err := Open(ctx, path)
	if err != nil {
		return nil, err
	}
	return requestAndClose(ctx, root)
}

func requestAndClose(ctx context.Context, root *Root) ([]byte, error) {
	raw, err := root.Request(ctx)
	if err := errors.Join(err, root.Close(), ctx.Err()); err != nil {
		return nil, err
	}
	return raw, nil
}

// Verification stays single-sourced while Connection owns the private binding caller.
func verifyResponse(raw []byte, request RequestView) (publication.Credential, error) {
	return publication.VerifyServiceResponse(raw, request)
}
