package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"
	"google.golang.org/protobuf/proto"
)

type grantRepository struct{ database storage.Database }

func (r grantRepository) recordGrant(ctx context.Context, artifact *Artifact, issuer ed25519.PublicKey, now time.Time) error {
	id, index, sum, record, err := prepareGrantRecord(artifact, issuer, now)
	if err != nil {
		return err
	}
	return r.database.Update(ctx, func(tx storage.WriteTransaction) error {
		return recordGrant(tx, id, index, sum, record)
	})
}

func prepareGrantRecord(artifact *Artifact, issuer ed25519.PublicKey, now time.Time) (string, []byte, [sha256.Size]byte, []byte, error) {
	if artifact == nil {
		return "", nil, [sha256.Size]byte{}, nil, errInvalid
	}
	raw, err := artifact.MarshalBinary()
	if err != nil {
		return "", nil, [sha256.Size]byte{}, nil, errInvalid
	}
	verified, err := ParseAndVerifyAccessGrant(raw, issuer, now)
	if err != nil || verified.ID() != artifact.ID() {
		return "", nil, [sha256.Size]byte{}, nil, errInvalid
	}
	payload := verified.AccessGrantPayload()
	index, err := grantIndexKey(payload)
	if err != nil {
		return "", nil, [sha256.Size]byte{}, nil, err
	}
	sum := sha256.Sum256(raw)
	record := append(append([]byte(nil), issuer...), raw...)
	return artifact.ID(), index, sum, record, nil
}

func recordGrant(tx storage.WriteTransaction, id string, index []byte, sum [sha256.Size]byte, record []byte) error {
	existing, found, err := tx.Get(grantsBucket, []byte(id))
	if err != nil {
		return err
	}
	if found && !bytes.Equal(existing, record) {
		return fmt.Errorf("conflicting Access Grant")
	}
	if !found {
		if err := tx.Put(grantsBucket, []byte(id), record); err != nil {
			return err
		}
	}
	return tx.Put(grantIndexBucket, index, sum[:])
}

func (r grantRepository) recordRevocation(ctx context.Context, artifact *Artifact, issuer ed25519.PublicKey, now time.Time) error {
	p := artifact.AccessGrantRevocationPayload()
	if p == nil {
		return errInvalid
	}
	var target *Artifact
	err := r.database.View(ctx, func(tx storage.ReadTransaction) error {
		loaded, loadErr := loadGrant(tx, p.TargetId, time.Time{})
		target = loaded
		return loadErr
	})
	if err != nil {
		return err
	}
	targetID, record, err := prepareGrantRevocation(artifact, issuer, now, target)
	if err != nil {
		return err
	}
	return r.database.Update(ctx, func(tx storage.WriteTransaction) error {
		return recordGrantRevocation(tx, targetID, record)
	})
}

func prepareGrantRevocation(artifact *Artifact, issuer ed25519.PublicKey, now time.Time, target *Artifact) (string, []byte, error) {
	if artifact == nil || target == nil {
		return "", nil, errInvalid
	}
	raw, err := artifact.MarshalBinary()
	if err != nil {
		return "", nil, errInvalid
	}
	verified, err := ParseAndVerifyAccessGrantRevocation(raw, issuer, now, target)
	if err != nil || verified.ID() != artifact.ID() {
		return "", nil, errInvalid
	}
	payload := verified.AccessGrantRevocationPayload()
	return payload.TargetId, append(append([]byte(nil), issuer...), raw...), nil
}

func recordGrantRevocation(tx storage.WriteTransaction, targetID string, record []byte) error {
	existing, found, err := tx.Get(grantRevocationsBucket, []byte(targetID))
	if err != nil {
		return err
	}
	if found && !bytes.Equal(existing, record) {
		return fmt.Errorf("conflicting Access Grant revocation")
	}
	if found {
		return nil
	}
	return tx.Put(grantRevocationsBucket, []byte(targetID), record)
}

func (r grantRepository) matches(ctx context.Context, now time.Time, subject string, audience Audience, action Action, resource ResourceRef) (bool, error) {
	prefix := grantIndexPrefix(audience, subject)
	var matched bool
	err := r.database.View(ctx, func(tx storage.ReadTransaction) error {
		var err error
		matched, err = grantMatches(tx, now, subject, audience, action, resource, prefix)
		return err
	})
	return matched, err
}

func grantMatches(tx storage.ReadTransaction, now time.Time, subject string, audience Audience, action Action, resource ResourceRef, prefix []byte) (bool, error) {
	matches, err := matchingGrantIDs(tx, now, subject, audience, action, resource, prefix)
	return len(matches) > 0, err
}

func matchingGrantIDs(tx storage.ReadTransaction, now time.Time, subject string, audience Audience, action Action, resource ResourceRef, prefix []byte) ([]string, error) {
	var matches []string
	err := tx.ForEach(grantIndexBucket, func(key, indexedHash []byte) error {
		if !bytes.HasPrefix(key, prefix) {
			return nil
		}
		id, err := grantIDFromIndex(key, prefix)
		if err != nil {
			return err
		}
		grant, err := loadGrant(tx, id, time.Time{})
		if err != nil {
			return err
		}
		raw, _ := grant.MarshalBinary()
		sum := sha256.Sum256(raw)
		if !bytes.Equal(indexedHash, sum[:]) {
			return fmt.Errorf("Access Grant index binding is corrupt")
		}
		payload := grant.AccessGrantPayload()
		expected, err := grantIndexKey(payload)
		if err != nil || !bytes.Equal(expected, key) {
			return fmt.Errorf("Access Grant index binding is corrupt")
		}
		if payload.Subject != subject || audienceFromProtocol(payload.Audience) != audience {
			return nil
		}
		if validateInterval(payload.NotBefore, payload.NotAfter, maxGrantLife, now) != nil {
			return nil
		}
		revoked, err := grantRevoked(tx, grant)
		if err != nil {
			return err
		}
		if revoked {
			return nil
		}
		hasAction := false
		for _, candidate := range payload.Actions {
			if candidate == string(action) {
				hasAction = true
				break
			}
		}
		if !hasAction {
			return nil
		}
		scope, err := scopeFromPayload(payload.Scope, audience.Node)
		if err != nil {
			return err
		}
		if !registeredActionAllowsScope(audience.Interface, action, scope.Kind) {
			return nil
		}
		if scope.Matches(resource, audience) {
			matches = append(matches, grant.ID())
		}
		return nil
	})
	return matches, err
}

func loadGrant(tx storage.ReadTransaction, id string, now time.Time) (*Artifact, error) {
	record, found, err := tx.Get(grantsBucket, []byte(id))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("Access Grant is missing")
	}
	if len(record) <= ed25519.PublicKeySize {
		return nil, fmt.Errorf("Access Grant record is corrupt")
	}
	issuer := ed25519.PublicKey(record[:ed25519.PublicKeySize])
	artifact, err := ParseAndVerifyAccessGrant(record[ed25519.PublicKeySize:], issuer, now)
	if err != nil || artifact.ID() != id {
		return nil, fmt.Errorf("Access Grant record is corrupt")
	}
	return artifact, nil
}
func grantRevoked(tx storage.ReadTransaction, grant *Artifact) (bool, error) {
	record, found, err := tx.Get(grantRevocationsBucket, []byte(grant.ID()))
	if err != nil || !found {
		return found, err
	}
	if len(record) <= ed25519.PublicKeySize {
		return false, fmt.Errorf("Access Grant revocation is corrupt")
	}
	_, err = ParseAndVerifyAccessGrantRevocation(record[ed25519.PublicKeySize:], ed25519.PublicKey(record[:ed25519.PublicKeySize]), time.Time{}, grant)
	if err != nil {
		return false, fmt.Errorf("Access Grant revocation is corrupt")
	}
	return true, nil
}

func grantIndexKey(payload *identityprotocol.AccessGrantPayload) ([]byte, error) {
	if payload == nil || payload.Audience == nil {
		return nil, errInvalid
	}
	aud := audienceFromProtocol(payload.Audience)
	prefix := grantIndexPrefix(aud, payload.Subject)
	return appendTuple(prefix, []byte(artifactIDFromGrantPayload(payload))), nil
}
func artifactIDFromGrantPayload(payload *identityprotocol.AccessGrantPayload) string {
	raw, _ := protoDeterministic(payload)
	return artifactID("ag1_", append([]byte(grantDomain), raw...))
}
func grantIndexPrefix(audience Audience, subject string) []byte {
	return tuple([]byte(audience.Node), []byte{byte(audience.Interface)}, uint32Bytes(audience.ProtocolMajor), []byte(subject))
}
func grantIDFromIndex(key, prefix []byte) (string, error) {
	if !bytes.HasPrefix(key, prefix) || len(key) < len(prefix)+2 {
		return "", fmt.Errorf("Access Grant index key is corrupt")
	}
	size := int(binary.BigEndian.Uint16(key[len(prefix) : len(prefix)+2]))
	if size == 0 || len(key) != len(prefix)+2+size {
		return "", fmt.Errorf("Access Grant index key is corrupt")
	}
	return string(key[len(prefix)+2:]), nil
}
func audienceFromProtocol(a *identityprotocol.Audience) Audience {
	if a == nil {
		return Audience{}
	}
	return Audience{Node: a.Node, Interface: a.Interface, ProtocolMajor: a.ProtocolMajor}
}
func tuple(parts ...[]byte) []byte {
	out := []byte{}
	for _, part := range parts {
		out = appendTuple(out, part)
	}
	return out
}
func appendTuple(out, part []byte) []byte {
	var size [2]byte
	binary.BigEndian.PutUint16(size[:], uint16(len(part)))
	out = append(out, size[:]...)
	return append(out, part...)
}
func uint32Bytes(value uint32) []byte {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], value)
	return raw[:]
}
func protoDeterministic(message *identityprotocol.AccessGrantPayload) ([]byte, error) {
	return proto.MarshalOptions{Deterministic: true}.Marshal(message)
}
