package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"fmt"

	identitycontract "ardents/api/ardents/identity/v1"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var initialOperatorRecoveryActions = []string{
	"identity.device-revocations.list",
	"identity.device.revoke",
	"identity.grant.issue",
	"identity.grant.list",
	"identity.grant.revoke",
	"identity.principal.enroll",
}

// AccessGrantIssuer is a typed Node-key operation, not a generic signing oracle.
type AccessGrantIssuer interface {
	PublicKey() ed25519.PublicKey
	IssueAccessGrant(*identityprotocol.AccessGrantPayload) (*Artifact, error)
}

type AccessGrantRevocationIssuer interface {
	IssueAccessGrantRevocation(*identityprotocol.AccessGrantRevocationPayload, *Artifact) (*Artifact, error)
}

type DeviceRevocationIssuer interface {
	IssueDeviceRevocation(*identityprotocol.DeviceRevocationPayload) (*Artifact, error)
}

type FirstEnrollmentRequest struct {
	Ticket        BootstrapTicket
	Challenge     Challenge
	Proof         EnrollmentProof
	RootPublicKey [ed25519.PublicKeySize]byte
	Credential    []byte
}

func (FirstEnrollmentRequest) String() string   { return "first Principal enrollment [redacted]" }
func (FirstEnrollmentRequest) GoString() string { return "first Principal enrollment [redacted]" }
func (FirstEnrollmentRequest) MarshalJSON() ([]byte, error) {
	return []byte(`{"protected":"[redacted]"}`), nil
}

type FirstEnrollmentResult struct {
	Principal    string
	CredentialID string
	GrantID      string
}

func (s *Service) EnrollFirstPrincipal(ctx context.Context, currentBinding AuthenticationBinding, request FirstEnrollmentRequest) (FirstEnrollmentResult, error) {
	audit := newLifecycleAudit(currentBinding.Audience, Action("identity.principal.enroll"))
	succeeded := false
	defer func() {
		if !succeeded {
			audit.recordDenied(s, "first_principal_enrollment_denied")
		}
	}()
	if !s.bootstrapEnabled {
		return FirstEnrollmentResult{}, ErrFeatureDisabled
	}
	if s.grantIssuer == nil {
		return FirstEnrollmentResult{}, ErrUnavailable
	}
	now := canonicalNow(s.clock.Now())
	if currentBinding != request.Challenge.Binding || validateAuthenticationBinding(currentBinding) != nil ||
		request.Challenge.Purpose != identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF ||
		request.Challenge.Binding.Audience.Interface != identityprotocol.Interface_INTERFACE_OPERATOR ||
		request.Challenge.Binding.Audience.ProtocolMajor != identitycontract.ProtocolMajor {
		return FirstEnrollmentResult{}, ErrInvalidArgument
	}
	principal, err := identityprincipal.FromEd25519PublicKey(request.RootPublicKey[:])
	if err != nil || principal.String() != request.Challenge.Principal {
		return FirstEnrollmentResult{}, ErrInvalidArgument
	}
	credential, err := ParseAndVerifyKeyCredential(request.Credential, now)
	if err != nil {
		return FirstEnrollmentResult{}, ErrInvalidArgument
	}
	credentialPayload := credential.KeyCredentialPayload()
	if credentialPayload.Subject != principal.String() || !bytes.Equal(credentialPayload.RootPublicKey, request.RootPublicKey[:]) {
		return FirstEnrollmentResult{}, ErrInvalidArgument
	}
	audit.identify(principal.String(), credentialPayload.DeviceId)
	if !s.consumeEnrollmentProof(request.Proof, request.Challenge) {
		return FirstEnrollmentResult{}, ErrUnauthenticated
	}
	issuerPublic := append(ed25519.PublicKey(nil), s.grantIssuer.PublicKey()...)
	node, err := identityprincipal.FromEd25519PublicKey(issuerPublic)
	if err != nil || node.String() != request.Challenge.Binding.Audience.Node {
		return FirstEnrollmentResult{}, ErrUnavailable
	}
	payload := &identityprotocol.AccessGrantPayload{
		Version:  identitycontract.Version,
		Issuer:   node.String(),
		Subject:  principal.String(),
		Audience: protocolAudience(request.Challenge.Binding.Audience),
		Actions:  append([]string(nil), initialOperatorRecoveryActions...),
		Scope: &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Node{
			Node: &identityprotocol.NodeScope{},
		}},
		NotBefore: timestamppb.New(now),
		NotAfter:  timestamppb.New(now.Add(identitycontract.DefaultGrantLifetime)),
	}
	grant, err := s.grantIssuer.IssueAccessGrant(proto.Clone(payload).(*identityprotocol.AccessGrantPayload))
	if err != nil || grant == nil || !proto.Equal(grant.AccessGrantPayload(), payload) {
		return FirstEnrollmentResult{}, ErrUnavailable
	}
	grantID, grantIndex, grantHash, grantRecord, err := prepareGrantRecord(grant, issuerPublic, now)
	if err != nil {
		return FirstEnrollmentResult{}, ErrUnavailable
	}
	var root [ed25519.PublicKeySize]byte
	copy(root[:], request.RootPublicKey[:])
	enrollment := EnrollmentRecord{Node: node.String(), Principal: principal.String(), RootPublicKey: root, EnrolledAt: now}
	enrollmentKey, enrollmentRecord, err := encodeEnrollment(enrollment)
	if err != nil {
		return FirstEnrollmentResult{}, ErrInvalidArgument
	}
	credentialKey, credentialRecord, err := prepareEnrollmentCredential(node.String(), principal.String(), credential, now)
	if err != nil {
		return FirstEnrollmentResult{}, ErrInvalidArgument
	}
	err = s.grants.database.Update(ctx, func(tx storage.WriteTransaction) error {
		if err := consumeBootstrapTicket(tx, node.String(), request.Ticket, now); err != nil {
			return err
		}
		if err := recordEnrollment(tx, enrollmentKey, enrollmentRecord); err != nil {
			return err
		}
		if err := recordEnrollmentCredential(tx, credentialKey, credentialRecord); err != nil {
			return err
		}
		if err := recordGrant(tx, grantID, grantIndex, grantHash, grantRecord); err != nil {
			return err
		}
		return audit.commitSuccessfulMutation(tx, "first_principal_enrolled", grantID)
	})
	if err != nil {
		if err == ErrUnauthenticated || err == ErrConflict {
			return FirstEnrollmentResult{}, err
		}
		return FirstEnrollmentResult{}, ErrUnavailable
	}
	succeeded = true
	if err := s.flushAuditOutbox(ctx); err != nil {
		return FirstEnrollmentResult{}, ErrUnavailable
	}
	return FirstEnrollmentResult{Principal: principal.String(), CredentialID: credential.ID(), GrantID: grant.ID()}, nil
}

var _ fmt.Stringer = FirstEnrollmentRequest{}
