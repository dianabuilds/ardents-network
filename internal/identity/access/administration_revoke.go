package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RevokeGrantRequest struct {
	Command AdminCommand
	GrantID string
}

type RevokeDeviceRequest struct {
	Command  AdminCommand
	Subject  string
	DeviceID string
}

func (s *Service) RevokeAccessGrant(ctx context.Context, request RevokeGrantRequest) (string, error) {
	succeeded := false
	defer func() {
		if !succeeded {
			s.record("denied", "admin_revoke_grant_denied", "", "", request.Command.Attempt.Binding.Audience)
		}
	}()
	revoker, ok := s.grantIssuer.(AccessGrantRevocationIssuer)
	if !ok || validateAdminCommand(request.Command, "identity.grant.revoke", "access-grant") != nil || request.Command.Attempt.Resource.ID != request.GrantID {
		return "", ErrInvalidArgument
	}
	now := canonicalNow(s.clock.Now())
	issuerPublic := append(ed25519.PublicKey(nil), s.grantIssuer.PublicKey()...)
	node, err := identityprincipal.FromEd25519PublicKey(issuerPublic)
	if err != nil || node.String() != request.Command.Attempt.Binding.Audience.Node {
		return "", ErrUnavailable
	}
	var target *Artifact
	err = s.grants.database.View(ctx, func(tx storage.ReadTransaction) error {
		var loadErr error
		target, loadErr = loadGrant(tx, request.GrantID, time.Time{})
		return loadErr
	})
	if err != nil {
		return "", ErrUnavailable
	}
	payload := &identityprotocol.AccessGrantRevocationPayload{Version: identitycontract.Version, TargetId: request.GrantID, Issuer: node.String(), Audience: protocolAudience(audienceFromProtocol(target.AccessGrantPayload().Audience)), RevokedAt: timestamppb.New(now)}
	revocation, err := revoker.IssueAccessGrantRevocation(proto.Clone(payload).(*identityprotocol.AccessGrantRevocationPayload), target)
	if err != nil || revocation == nil || !proto.Equal(revocation.AccessGrantRevocationPayload(), payload) {
		return "", ErrUnavailable
	}
	targetID, record, err := prepareGrantRevocation(revocation, issuerPublic, now, target)
	if err != nil {
		return "", ErrUnavailable
	}
	digest := stableAdminDigest("revoke-grant", []byte(request.GrantID))
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	result := ""
	var actor, device string
	err = s.grants.database.Update(ctx, func(tx storage.WriteTransaction) error {
		transactionNow := canonicalNow(s.clock.Now())
		call, session, admitErr := s.admitInTransaction(tx, transactionNow, request.Command.Attempt)
		if admitErr != nil {
			return admitErr
		}
		actor, device = call.Actor(), session.DeviceID
		key := adminCommandKey(node.String(), actor, string(call.Action()), request.Command.RequestID)
		prior, found, commandErr := loadAdminCommand(tx, key, digest, identitycontract.AccessGrantRevocationPrefix)
		if commandErr != nil {
			return commandErr
		}
		if found {
			result = prior
			return nil
		}
		current, loadErr := loadGrant(tx, targetID, time.Time{})
		if loadErr != nil || current.ID() != target.ID() {
			return fmt.Errorf("Access Grant changed before revocation")
		}
		if isRecoveryGrant(current, node.String(), transactionNow) {
			available, guardErr := hasRecoveryPath(tx, node.String(), transactionNow, targetID, "", "")
			if guardErr != nil {
				return guardErr
			}
			if !available {
				return ErrConflict
			}
		}
		if err := recordGrantRevocation(tx, targetID, record); err != nil {
			return err
		}
		result = revocation.ID()
		return recordAdminCommand(tx, key, digest, result, identitycontract.AccessGrantRevocationPrefix)
	})
	if err != nil {
		return "", mapAdminError(err)
	}
	s.record("accepted", "access_grant_revoked", actor, device, request.Command.Attempt.Binding.Audience)
	succeeded = true
	return result, nil
}

func (s *Service) RevokeDevice(ctx context.Context, request RevokeDeviceRequest) (string, error) {
	succeeded := false
	defer func() {
		if !succeeded {
			s.record("denied", "admin_revoke_device_denied", "", "", request.Command.Attempt.Binding.Audience)
		}
	}()
	issuer, ok := s.grantIssuer.(DeviceRevocationIssuer)
	if !ok || validateAdminCommand(request.Command, "identity.device.revoke", "device") != nil {
		return "", ErrInvalidArgument
	}
	if _, err := identityprincipal.Parse(request.Subject); err != nil {
		return "", ErrInvalidArgument
	}
	deviceResourceID, resourceErr := DeviceResourceID(request.Subject, request.DeviceID)
	if resourceErr != nil || request.Command.Attempt.Resource.ID != deviceResourceID {
		return "", ErrInvalidArgument
	}
	now := canonicalNow(s.clock.Now())
	public := append(ed25519.PublicKey(nil), s.grantIssuer.PublicKey()...)
	node, err := identityprincipal.FromEd25519PublicKey(public)
	if err != nil || node.String() != request.Command.Attempt.Binding.Audience.Node {
		return "", ErrUnavailable
	}
	payload := &identityprotocol.DeviceRevocationPayload{Version: identitycontract.Version, TargetId: request.DeviceID, Issuer: node.String(), Audience: protocolAudience(request.Command.Attempt.Binding.Audience), RevokedAt: timestamppb.New(now), TargetDeviceId: request.DeviceID, Subject: request.Subject}
	artifact, err := issuer.IssueDeviceRevocation(proto.Clone(payload).(*identityprotocol.DeviceRevocationPayload))
	if err != nil || artifact == nil || !proto.Equal(artifact.DeviceRevocationPayload(), payload) {
		return "", ErrUnavailable
	}
	raw, err := artifact.MarshalBinary()
	if err != nil {
		return "", ErrUnavailable
	}
	verified, err := ParseAndVerifyDeviceRevocation(raw, public, now)
	if err != nil || verified.ID() != artifact.ID() {
		return "", ErrUnavailable
	}
	revocationKey, err := deviceRevocationKey(node.String(), request.Subject, request.DeviceID)
	if err != nil {
		return "", ErrInvalidArgument
	}
	digest := stableAdminDigest("revoke-device", []byte(request.Subject), []byte(request.DeviceID))
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	result := ""
	var actor, device string
	err = s.grants.database.Update(ctx, func(tx storage.WriteTransaction) error {
		transactionNow := canonicalNow(s.clock.Now())
		call, session, admitErr := s.admitInTransaction(tx, transactionNow, request.Command.Attempt)
		if admitErr != nil {
			return admitErr
		}
		actor, device = call.Actor(), session.DeviceID
		key := adminCommandKey(node.String(), actor, string(call.Action()), request.Command.RequestID)
		prior, found, commandErr := loadAdminCommand(tx, key, digest, identitycontract.DeviceRevocationPrefix)
		if commandErr != nil {
			return commandErr
		}
		if found {
			result = prior
			return nil
		}
		available, guardErr := hasRecoveryPath(tx, node.String(), transactionNow, "", request.Subject, request.DeviceID)
		if guardErr != nil {
			return guardErr
		}
		if !available {
			return ErrConflict
		}
		if err := recordDeviceRevocation(tx, revocationKey, raw); err != nil {
			return err
		}
		result = artifact.ID()
		return recordAdminCommand(tx, key, digest, result, identitycontract.DeviceRevocationPrefix)
	})
	if err != nil {
		return "", mapAdminError(err)
	}
	s.sessions.invalidateDevice(request.DeviceID)
	s.record("accepted", "device_revoked", actor, device, request.Command.Attempt.Binding.Audience)
	succeeded = true
	return result, nil
}

func stableAdminDigest(domain string, parts ...[]byte) [sha256.Size]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents:admin:" + domain + ":v1\x00"))
	_, _ = hash.Write(tuple(parts...))
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}

func hasRecoveryPath(tx storage.ReadTransaction, node string, now time.Time, excludedGrant, excludedSubject, excludedDevice string) (bool, error) {
	found := false
	err := tx.ForEach(grantsBucket, func(key, _ []byte) error {
		if string(key) == excludedGrant {
			return nil
		}
		grant, err := loadGrant(tx, string(key), time.Time{})
		if err != nil {
			return err
		}
		if !isRecoveryGrant(grant, node, now) {
			return nil
		}
		revoked, err := grantRevoked(tx, grant)
		if err != nil || revoked {
			return err
		}
		hasDevice, err := subjectHasRecoveryDevice(tx, node, grant.AccessGrantPayload().Subject, now, excludedSubject, excludedDevice)
		if hasDevice {
			found = true
		}
		return err
	})
	return found, err
}

func isRecoveryGrant(grant *Artifact, node string, now time.Time) bool {
	payload := grant.AccessGrantPayload()
	if payload == nil || payload.Audience == nil || payload.Audience.Node != node || payload.Audience.Interface != identityprotocol.Interface_INTERFACE_OPERATOR || payload.Audience.ProtocolMajor != identitycontract.ProtocolMajor || validateInterval(payload.NotBefore, payload.NotAfter, maxGrantLife, now) != nil {
		return false
	}
	if _, ok := payload.Scope.Scope.(*identityprotocol.ResourceScope_Node); !ok {
		return false
	}
	for _, required := range initialOperatorRecoveryActions {
		present := false
		for _, action := range payload.Actions {
			if action == required {
				present = true
				break
			}
		}
		if !present {
			return false
		}
	}
	return true
}

func subjectHasRecoveryDevice(tx storage.ReadTransaction, node, subject string, now time.Time, excludedSubject, excludedDevice string) (bool, error) {
	prefix := tuple([]byte(node), []byte(subject))
	found := false
	err := tx.ForEach(enrollmentCredentialsBucket, func(key, raw []byte) error {
		if !bytes.HasPrefix(key, prefix) {
			return nil
		}
		credential, err := loadEnrollmentCredential(raw, now)
		if err != nil {
			return err
		}
		payload := credential.KeyCredentialPayload()
		expected := tuple([]byte(node), []byte(subject), []byte(payload.DeviceId), []byte(credential.ID()))
		if !bytes.Equal(expected, key) || payload.Subject != subject {
			return fmt.Errorf("enrollment Credential index is corrupt")
		}
		if subject == excludedSubject && payload.DeviceId == excludedDevice {
			return nil
		}
		revocationKey, err := deviceRevocationKey(node, subject, payload.DeviceId)
		if err != nil {
			return err
		}
		revoked, err := deviceRevoked(tx, revocationKey)
		if err == nil && !revoked {
			found = true
		}
		return err
	})
	return found, err
}
