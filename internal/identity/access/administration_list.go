package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"sort"
	"time"

	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/storage"
)

type GrantMetadata struct {
	ID        string
	Subject   string
	Audience  Audience
	Actions   []Action
	Scope     ResourceScope
	NotBefore time.Time
	NotAfter  time.Time
	Revoked   bool
}

type DeviceRevocationMetadata struct {
	ID        string
	Subject   string
	DeviceID  string
	RevokedAt time.Time
}

func (s *Service) ListAccessGrants(ctx context.Context, attempt Attempt, subject string) ([]GrantMetadata, error) {
	audit := newAdministrationAudit(attempt)
	succeeded := false
	defer func() {
		if !succeeded {
			audit.recordDenied(s, "admin_list_grants_denied", attempt)
		}
	}()
	if string(attempt.Action) != "identity.grant.list" || string(attempt.Resource.Kind) != "grant-collection" || attempt.Resource.ID != subject {
		return nil, ErrInvalidArgument
	}
	if _, err := identityprincipal.Parse(subject); err != nil {
		return nil, ErrInvalidArgument
	}
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	result := []GrantMetadata{}
	err := s.grants.database.View(ctx, func(tx storage.ReadTransaction) error {
		if _, err := audit.admit(s, tx, canonicalNow(s.clock.Now()), attempt); err != nil {
			return err
		}
		return tx.ForEach(grantsBucket, func(key, _ []byte) error {
			grant, err := loadGrant(tx, string(key), time.Time{})
			if err != nil {
				return err
			}
			payload := grant.AccessGrantPayload()
			if payload.Subject != subject || audienceFromProtocol(payload.Audience) != attempt.Binding.Audience {
				return nil
			}
			scope, err := scopeFromPayload(payload.Scope, payload.Audience.Node)
			if err != nil {
				return err
			}
			revoked, err := grantRevoked(tx, grant)
			if err != nil {
				return err
			}
			actions := make([]Action, len(payload.Actions))
			for index := range payload.Actions {
				actions[index] = Action(payload.Actions[index])
			}
			result = append(result, GrantMetadata{ID: grant.ID(), Subject: subject, Audience: attempt.Binding.Audience, Actions: actions, Scope: scope, NotBefore: payload.NotBefore.AsTime(), NotAfter: payload.NotAfter.AsTime(), Revoked: revoked})
			return nil
		})
	})
	if err != nil {
		return nil, mapAdminError(err)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	succeeded = true
	return result, nil
}

func (s *Service) ListDeviceRevocations(ctx context.Context, attempt Attempt, subject string) ([]DeviceRevocationMetadata, error) {
	audit := newAdministrationAudit(attempt)
	succeeded := false
	defer func() {
		if !succeeded {
			audit.recordDenied(s, "admin_list_device_revocations_denied", attempt)
		}
	}()
	if string(attempt.Action) != "identity.device-revocations.list" || string(attempt.Resource.Kind) != "device-revocation-collection" || attempt.Resource.ID != subject {
		return nil, ErrInvalidArgument
	}
	if _, err := identityprincipal.Parse(subject); err != nil {
		return nil, ErrInvalidArgument
	}
	if s.grantIssuer == nil {
		return nil, ErrUnavailable
	}
	public := append(ed25519.PublicKey(nil), s.grantIssuer.PublicKey()...)
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	result := []DeviceRevocationMetadata{}
	err := s.grants.database.View(ctx, func(tx storage.ReadTransaction) error {
		if _, err := audit.admit(s, tx, canonicalNow(s.clock.Now()), attempt); err != nil {
			return err
		}
		prefix := tuple([]byte(attempt.Binding.Audience.Node), []byte(subject))
		return tx.ForEach(deviceRevocationsBucket, func(key, raw []byte) error {
			if !bytes.HasPrefix(key, prefix) {
				return nil
			}
			artifact, err := ParseAndVerifyDeviceRevocation(raw, public, time.Time{})
			if err != nil {
				return err
			}
			payload := artifact.DeviceRevocationPayload()
			expected, keyErr := deviceRevocationKey(payload.Audience.Node, payload.Subject, payload.TargetDeviceId)
			if keyErr != nil || !bytes.Equal(expected, key) || payload.Audience.Node != attempt.Binding.Audience.Node || payload.Subject != subject {
				return ErrUnavailable
			}
			result = append(result, DeviceRevocationMetadata{ID: artifact.ID(), Subject: payload.Subject, DeviceID: payload.TargetDeviceId, RevokedAt: payload.RevokedAt.AsTime()})
			return nil
		})
	})
	if err != nil {
		return nil, mapAdminError(err)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].DeviceID < result[j].DeviceID })
	succeeded = true
	return result, nil
}
