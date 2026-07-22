package identity

import (
	"context"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"
	identityprotocol "ardents/internal/identity/protocol"

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
var _ EnrollmentSigner = (*RootFileSigner)(nil)
