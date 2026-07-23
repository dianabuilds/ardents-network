package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"

	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"
)

type EnrollPrincipalRequest struct {
	Command       AdminCommand
	Challenge     Challenge
	Proof         EnrollmentProof
	RootPublicKey [ed25519.PublicKeySize]byte
	Credential    []byte
}

func (EnrollPrincipalRequest) String() string   { return "enroll Principal request [redacted]" }
func (EnrollPrincipalRequest) GoString() string { return "enroll Principal request [redacted]" }
func (EnrollPrincipalRequest) MarshalJSON() ([]byte, error) {
	return []byte(`{"protected":"[redacted]"}`), nil
}

func (s *Service) EnrollPrincipal(ctx context.Context, request EnrollPrincipalRequest) (string, error) {
	audit := newAdministrationAudit(request.Command.Attempt)
	succeeded := false
	defer func() {
		if !succeeded {
			audit.recordDenied(s, "admin_enroll_principal_denied", request.Command.Attempt)
		}
	}()
	if validateAdminCommand(request.Command, "identity.principal.enroll", "principal") != nil || request.Challenge.Purpose != identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF || request.Challenge.Binding.Audience.Interface != identityprotocol.Interface_INTERFACE_OPERATOR {
		return "", ErrInvalidArgument
	}
	now := canonicalNow(s.clock.Now())
	principal, err := identityprincipal.FromEd25519PublicKey(request.RootPublicKey[:])
	if err != nil || principal.String() != request.Challenge.Principal || request.Command.Attempt.Resource.ID != principal.String() || request.Challenge.Binding != request.Command.Attempt.Binding {
		return "", ErrInvalidArgument
	}
	credential, err := ParseAndVerifyKeyCredential(request.Credential, now)
	if err != nil {
		return "", ErrInvalidArgument
	}
	payload := credential.KeyCredentialPayload()
	if payload.Subject != principal.String() || !bytes.Equal(payload.RootPublicKey, request.RootPublicKey[:]) {
		return "", ErrInvalidArgument
	}
	node := request.Command.Attempt.Binding.Audience.Node
	enrollment := EnrollmentRecord{Node: node, Principal: principal.String(), RootPublicKey: request.RootPublicKey, EnrolledAt: now}
	enrollmentKey, enrollmentRecord, err := encodeEnrollment(enrollment)
	if err != nil {
		return "", ErrInvalidArgument
	}
	credentialKey, credentialRecord, err := prepareEnrollmentCredential(node, principal.String(), credential, now)
	if err != nil {
		return "", ErrInvalidArgument
	}
	challengeHash, err := challengeDigest(request.Challenge)
	if err != nil {
		return "", ErrInvalidArgument
	}
	digest := sha256.Sum256(append(append([]byte("ardents:admin:enroll-principal:v1\x00"), credentialRecord...), challengeHash[:]...))
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	result := ""
	err = s.grants.database.Update(ctx, func(tx storage.WriteTransaction) error {
		transactionNow := canonicalNow(s.clock.Now())
		call, admitErr := audit.admit(s, tx, transactionNow, request.Command.Attempt)
		if admitErr != nil {
			return admitErr
		}
		key := adminCommandKey(node, call.Actor(), string(call.Action()), request.Command.RequestID)
		prior, found, commandErr := loadAdminCommand(tx, key, digest, "p1_")
		if commandErr != nil {
			return commandErr
		}
		if found {
			result = prior
			return audit.commitSuccessfulMutation(tx, "principal_enrolled")
		}
		if !s.consumeEnrollmentProof(request.Proof, request.Challenge) {
			return ErrUnauthenticated
		}
		if _, credentialErr := loadEnrollmentCredential(credentialRecord, transactionNow); credentialErr != nil {
			return ErrInvalidArgument
		}
		if _, enrolled, readErr := tx.Get(enrollmentsBucket, enrollmentKey); readErr != nil {
			return readErr
		} else if enrolled {
			return ErrConflict
		}
		if err := recordEnrollment(tx, enrollmentKey, enrollmentRecord); err != nil {
			return err
		}
		if err := recordEnrollmentCredential(tx, credentialKey, credentialRecord); err != nil {
			return err
		}
		result = principal.String()
		if err := recordAdminCommand(tx, key, digest, result, "p1_"); err != nil {
			return err
		}
		return audit.commitSuccessfulMutation(tx, "principal_enrolled")
	})
	if err != nil {
		return "", mapAdminError(err)
	}
	succeeded = true
	if err := s.flushAuditOutbox(ctx); err != nil {
		return "", ErrUnavailable
	}
	return result, nil
}
