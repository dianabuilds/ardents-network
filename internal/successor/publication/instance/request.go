package instance

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
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

// Verification stays single-sourced while Connection owns the private binding caller.
func verifyResponse(raw []byte, request RequestView) (publication.Credential, error) {
	return publication.VerifyServiceResponse(raw, request)
}
