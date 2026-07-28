// Package channeldelivery owns recipient-Node initial-generation preparation
// and installation. It does not own Authority truth, Operator authentication,
// Product Policy, or the HPKE implementation.
package channeldelivery

import (
	"context"
	"crypto/ed25519"
	"errors"
	"time"

	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
)

const (
	ContractVersion            = 1
	MaximumAttestationValidity = 30 * 24 * time.Hour
)

var (
	ErrInvalidArgument    = errors.New("channel delivery argument is invalid")
	ErrPermissionDenied   = errors.New("channel delivery permission denied")
	ErrUnavailable        = errors.New("channel delivery is unavailable")
	ErrUnsupportedVersion = errors.New("channel delivery version is unsupported")
)

type Command struct {
	Actor, Effective string
}

type PrepareRequest struct {
	Version          uint32
	SubjectPrincipal string
	ValidFor         time.Duration
}

type Service struct {
	capabilities *identitycapability.Service
	identityKey  ed25519.PrivateKey
	clock        func() time.Time
	subject      string
}

func New(
	capabilities *identitycapability.Service,
	identityKey ed25519.PrivateKey,
	subject string,
	clock func() time.Time,
) (*Service, error) {
	if capabilities == nil || len(identityKey) != ed25519.PrivateKeySize || subject == "" {
		return nil, ErrUnavailable
	}
	if clock == nil {
		clock = time.Now
	}
	return &Service{
		capabilities: capabilities,
		identityKey:  append(ed25519.PrivateKey(nil), identityKey...),
		clock:        clock,
		subject:      subject,
	}, nil
}

func (s *Service) Prepare(
	_ context.Context,
	command Command,
	request PrepareRequest,
) (identityapi.CapabilityDeliveryAttestation, error) {
	if request.Version != ContractVersion {
		return identityapi.CapabilityDeliveryAttestation{}, ErrUnsupportedVersion
	}
	if command.Actor == "" || command.Actor != command.Effective ||
		request.SubjectPrincipal != s.subject {
		return identityapi.CapabilityDeliveryAttestation{}, ErrPermissionDenied
	}
	if request.ValidFor <= 0 || request.ValidFor > MaximumAttestationValidity ||
		request.ValidFor%time.Second != 0 {
		return identityapi.CapabilityDeliveryAttestation{}, ErrInvalidArgument
	}
	attestation, err := s.capabilities.AttestDeliveryPublicKey(
		s.identityKey, s.clock().UTC().Truncate(time.Second).Add(request.ValidFor),
	)
	if err != nil {
		return identityapi.CapabilityDeliveryAttestation{}, ErrUnavailable
	}
	return attestation, nil
}

func (s *Service) Install(
	_ context.Context,
	command Command,
	version uint32,
	sealed identitycapability.SealedGenerationDelivery,
) (identitycapability.GenerationDeliveryReceipt, error) {
	if version != ContractVersion {
		return identitycapability.GenerationDeliveryReceipt{}, ErrUnsupportedVersion
	}
	if command.Actor == "" || command.Actor != command.Effective ||
		sealed.Binding.RecipientPrincipal != s.subject {
		return identitycapability.GenerationDeliveryReceipt{}, ErrPermissionDenied
	}
	receipt, err := s.capabilities.InstallGenerationDelivery(sealed)
	if err != nil {
		return identitycapability.GenerationDeliveryReceipt{}, ErrInvalidArgument
	}
	return receipt, nil
}
