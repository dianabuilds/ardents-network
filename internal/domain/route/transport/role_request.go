package transport

import (
	"context"
	"errors"
	"time"
)

// ClosedRoleCarrierRequest selects one direct role transport without a stable
// client certificate, DNS, root pool, retry or profile fallback.
type ClosedRoleCarrierRequest struct {
	CarrierProfile CarrierProfile
	Endpoint       string
	ExpectedServer [32]byte
	Deadline       time.Time
}

// ValidateRoleRequest applies shared pre-output bounds to the TLS and QUIC
// direct role adapters. It supplies neither authenticated State nor a token.
func ValidateRoleRequest(ctx context.Context, input ClosedRoleCarrierRequest) error {
	if ctx == nil || !LiteralEndpoint(input.Endpoint) || input.ExpectedServer == [32]byte{} || input.Deadline.IsZero() || !time.Now().Before(input.Deadline) {
		return errors.New("closed role carrier request is invalid")
	}
	if input.CarrierProfile != ClosedCarrierTCP && input.CarrierProfile != ClosedCarrierQUIC {
		return errors.New("closed role carrier profile is unsupported")
	}
	return nil
}
