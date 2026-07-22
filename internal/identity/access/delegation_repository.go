package access

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"google.golang.org/protobuf/proto"
)

var errDelegationRevocationConflict = errors.New("conflicting Delegation revocation")

type delegationRepository struct{ database storage.Database }

// ImportDelegationRevocation verifies and permanently records a canonical
// owner-issued revocation. The target Delegation need not have been seen.
func (s *Service) ImportDelegationRevocation(ctx context.Context, raw []byte) error {
	if len(raw) == 0 || len(raw) > maxArtifactBytes {
		return ErrInvalidArgument
	}
	revocation, err := ParseAndVerifyDelegationRevocation(raw, canonicalNow(s.clock.Now()))
	if err != nil {
		return ErrInvalidArgument
	}
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	if err := s.delegations.recordRevocation(ctx, revocation); err != nil {
		if errors.Is(err, errDelegationRevocationConflict) {
			return ErrConflict
		}
		return ErrUnavailable
	}
	return nil
}

func (r delegationRepository) recordRevocation(ctx context.Context, artifact *Artifact) error {
	if artifact == nil || artifact.DelegationRevocationPayload() == nil {
		return errInvalid
	}
	raw, err := artifact.MarshalBinary()
	if err != nil {
		return errInvalid
	}
	payload := artifact.DelegationRevocationPayload()
	return r.database.Update(ctx, func(tx storage.WriteTransaction) error {
		return recordDelegationRevocation(tx, artifact.ID(), payload.TargetId, raw)
	})
}

func recordDelegationRevocation(tx storage.WriteTransaction, revocationID, targetID string, raw []byte) error {
	byID, idFound, err := tx.Get(delegationRevocationIDsBucket, []byte(revocationID))
	if err != nil {
		return err
	}
	byTarget, targetFound, err := tx.Get(delegationRevocationsBucket, []byte(targetID))
	if err != nil {
		return err
	}
	if idFound && !bytes.Equal(byID, raw) || targetFound && !bytes.Equal(byTarget, raw) {
		return errDelegationRevocationConflict
	}
	if idFound && targetFound {
		return nil
	}
	if err := tx.Put(delegationRevocationIDsBucket, []byte(revocationID), raw); err != nil {
		return err
	}
	return tx.Put(delegationRevocationsBucket, []byte(targetID), raw)
}

func delegationRevoked(tx storage.ReadTransaction, delegation *Artifact) (bool, error) {
	if delegation == nil || delegation.DelegationPayload() == nil {
		return false, errInvalid
	}
	record, found, err := tx.Get(delegationRevocationsBucket, []byte(delegation.ID()))
	if err != nil || !found {
		return found, err
	}
	revocation, err := ParseAndVerifyDelegationRevocation(record, time.Time{})
	if err != nil {
		return false, fmt.Errorf("Delegation revocation is corrupt")
	}
	revoked := revocation.DelegationRevocationPayload()
	target := delegation.DelegationPayload()
	if revoked.TargetId != delegation.ID() || revoked.Delegator != target.Delegator || revoked.Delegatee != target.Delegatee || !proto.Equal(revoked.Audience, target.Audience) || !sameCredential(revoked.Credential, target.Credential) {
		return false, fmt.Errorf("Delegation revocation target binding is corrupt")
	}
	return true, nil
}

func sameCredential(left, right *identityprotocol.KeyCredential) bool {
	return left != nil && right != nil && proto.Equal(left, right)
}
