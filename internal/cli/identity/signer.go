package identity

import (
	"context"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"
	identityprotocol "ardents/internal/identity/protocol"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type KeyCredentialSpec struct {
	Subject         string
	RootPublicKey   []byte
	DeviceID        string
	DevicePublicKey []byte
	NotBefore       time.Time
	NotAfter        time.Time
}

// SessionSigner is the only routine-authentication signing surface exposed by
// the CLI. It cannot sign arbitrary bytes or mint identity artifacts.
type SessionSigner interface {
	Principal(context.Context) (string, error)
	Credential(context.Context) (*identityaccess.Artifact, error)
	SignAuthenticationChallenge(context.Context, identityaccess.Challenge) ([]byte, error)
}

// EnrollmentSigner is the root-key surface. It is intentionally separate from
// SessionSigner so normal authentication never needs to load the root key.
type EnrollmentSigner interface {
	Principal(context.Context) (string, error)
	IssueKeyCredential(context.Context, KeyCredentialSpec) (*identityaccess.Artifact, error)
	SignEnrollmentChallenge(context.Context, identityaccess.Challenge) ([]byte, error)
}

// DelegationSpec is the complete typed consent envelope a device may sign.
// It deliberately has no opaque payload or generic byte-signing escape hatch.
type DelegationSpec struct {
	Delegatee string
	Audience  identityaccess.Audience
	Actions   []identityaccess.Action
	Scope     identityaccess.ResourceScope
	NotBefore time.Time
	NotAfter  time.Time
}

// DelegationSigner is separate from routine session authentication. A caller
// must construct and display a typed consent proposal before invoking it.
type DelegationSigner interface {
	Principal(context.Context) (string, error)
	SignDelegation(context.Context, DelegationSpec, time.Time) (*identityaccess.Artifact, error)
}

// DelegationRevocationSigner can only revoke a concrete, verified Delegation
// with the same device Credential that signed it.
type DelegationRevocationSigner interface {
	Principal(context.Context) (string, error)
	SignDelegationRevocation(context.Context, *identityaccess.Artifact, time.Time) (*identityaccess.Artifact, error)
}

type DeviceFileSigner struct{ material deviceMaterial }

func (*DeviceFileSigner) String() string   { return "CLI device signer [redacted]" }
func (*DeviceFileSigner) GoString() string { return "CLI device signer [redacted]" }

func OpenDeviceFileSigner(path string) (*DeviceFileSigner, error) {
	material, err := loadDeviceMaterial(path)
	if err != nil {
		return nil, err
	}
	return &DeviceFileSigner{material: material}, nil
}

func (s *DeviceFileSigner) Principal(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return s.material.principal, nil
}

func (s *DeviceFileSigner) Credential(ctx context.Context) (*identityaccess.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.material.credential, nil
}

func (s *DeviceFileSigner) SignAuthenticationChallenge(ctx context.Context, challenge identityaccess.Challenge) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return identityaccess.SignAuthenticationChallenge(challenge, s.material.credential, s.material.private)
}

func (s *DeviceFileSigner) SignDelegation(ctx context.Context, spec DelegationSpec, now time.Time) (*identityaccess.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(spec.Actions) == 0 || len(spec.Actions) > identitycontract.MaxActions || spec.Scope.Kind == identityaccess.ScopePrincipalOwned && spec.Scope.Owner != s.material.principal {
		return nil, identityaccess.ErrInvalidArgument
	}
	seen := make(map[identityaccess.Action]struct{}, len(spec.Actions))
	for _, action := range spec.Actions {
		parsed, err := identityaccess.ParseAction(identityprotocol.Interface_INTERFACE_APPLICATION, string(action))
		if err != nil || parsed != action {
			return nil, identityaccess.ErrInvalidArgument
		}
		if _, duplicate := seen[action]; duplicate {
			return nil, identityaccess.ErrInvalidArgument
		}
		seen[action] = struct{}{}
	}
	scope, err := identityaccess.ResourceScopeFields(spec.Scope, spec.Audience)
	if err != nil {
		return nil, err
	}
	credentialRaw, err := s.material.credential.MarshalBinary()
	if err != nil {
		return nil, err
	}
	credential := new(identityprotocol.KeyCredential)
	if err := proto.Unmarshal(credentialRaw, credential); err != nil {
		return nil, err
	}
	actions := make([]string, len(spec.Actions))
	for index, action := range spec.Actions {
		actions[index] = string(action)
	}
	return identityaccess.SignDelegation(&identityprotocol.DelegationPayload{
		Version: identitycontract.Version, Delegator: s.material.principal, Delegatee: spec.Delegatee,
		Audience: &identityprotocol.Audience{Node: spec.Audience.Node, Interface: spec.Audience.Interface, ProtocolMajor: spec.Audience.ProtocolMajor},
		Actions:  actions, Scope: scope, NotBefore: timestamppb.New(spec.NotBefore.UTC()), NotAfter: timestamppb.New(spec.NotAfter.UTC()), Credential: credential,
	}, s.material.private, now.UTC())
}

func (s *DeviceFileSigner) SignDelegationRevocation(ctx context.Context, delegation *identityaccess.Artifact, revokedAt time.Time) (*identityaccess.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	payload := delegation.DelegationPayload()
	if payload == nil || payload.Delegator != s.material.principal || payload.Credential == nil {
		return nil, identityaccess.ErrInvalidArgument
	}
	credentialRaw, err := s.material.credential.MarshalBinary()
	if err != nil {
		return nil, identityaccess.ErrInvalidArgument
	}
	credential := new(identityprotocol.KeyCredential)
	if err := proto.Unmarshal(credentialRaw, credential); err != nil || !proto.Equal(payload.Credential, credential) {
		return nil, identityaccess.ErrInvalidArgument
	}
	return identityaccess.SignDelegationRevocation(&identityprotocol.DelegationRevocationPayload{
		Version: identitycontract.Version, TargetId: delegation.ID(), Issuer: payload.Delegator,
		Audience: proto.Clone(payload.Audience).(*identityprotocol.Audience), RevokedAt: timestamppb.New(revokedAt.UTC()),
		Delegator: payload.Delegator, Delegatee: payload.Delegatee, Credential: credential,
	}, s.material.private, revokedAt.UTC())
}

type RootFileSigner struct{ material rootMaterial }

func (*RootFileSigner) String() string   { return "CLI root signer [redacted]" }
func (*RootFileSigner) GoString() string { return "CLI root signer [redacted]" }

func OpenRootFileSigner(path string) (*RootFileSigner, error) {
	material, err := loadRootMaterial(path)
	if err != nil {
		return nil, err
	}
	return &RootFileSigner{material: material}, nil
}

func (s *RootFileSigner) Principal(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return s.material.principal, nil
}

func (s *RootFileSigner) IssueKeyCredential(ctx context.Context, spec KeyCredentialSpec) (*identityaccess.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return identityaccess.SignKeyCredential(&identityprotocol.KeyCredentialPayload{
		Version: identitycontract.Version, Subject: spec.Subject,
		RootPublicKey: append([]byte(nil), spec.RootPublicKey...), DeviceId: spec.DeviceID,
		DevicePublicKey: append([]byte(nil), spec.DevicePublicKey...),
		Purposes:        []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE},
		NotBefore:       timestamppb.New(spec.NotBefore.UTC()), NotAfter: timestamppb.New(spec.NotAfter.UTC()),
	}, s.material.private)
}

func (s *RootFileSigner) SignEnrollmentChallenge(ctx context.Context, challenge identityaccess.Challenge) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return identityaccess.SignEnrollmentChallenge(challenge, s.material.private)
}

var _ SessionSigner = (*DeviceFileSigner)(nil)
var _ DelegationSigner = (*DeviceFileSigner)(nil)
var _ DelegationRevocationSigner = (*DeviceFileSigner)(nil)
var _ EnrollmentSigner = (*RootFileSigner)(nil)
