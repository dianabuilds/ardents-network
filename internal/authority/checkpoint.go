package authority

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	identityprincipal "ardents/internal/identity/principal"
)

const checkpointDomain = "ardents:realm-authority-checkpoint:v1\x00"

type Signer interface {
	Principal(context.Context) (string, error)
	PublicKey(context.Context) (ed25519.PublicKey, error)
	Sign(context.Context, []byte) ([]byte, error)
}

type Checkpoint struct {
	Version            uint32    `json:"version"`
	SchemaVersion      uint32    `json:"schema_version"`
	RealmID            string    `json:"realm_id"`
	AuthorityPrincipal string    `json:"authority_principal"`
	AuthorityPublicKey []byte    `json:"authority_public_key"`
	AuthorityEpoch     uint64    `json:"authority_epoch"`
	AuthoritySequence  uint64    `json:"authority_sequence"`
	PreviousDigest     string    `json:"previous_digest,omitempty"`
	AuditHead          string    `json:"audit_head"`
	CreatedAt          time.Time `json:"created_at"`
}

type SignedCheckpoint struct {
	Checkpoint
	Digest    string `json:"digest"`
	Signature []byte `json:"signature"`
}

func checkpointMessage(body Checkpoint) ([]byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return append([]byte(checkpointDomain), raw...), nil
}

func SignCheckpoint(ctx context.Context, signer Signer, body Checkpoint) (SignedCheckpoint, error) {
	if signer == nil {
		return SignedCheckpoint{}, ErrUnavailable
	}
	message, err := checkpointMessage(body)
	if err != nil {
		return SignedCheckpoint{}, ErrInvalidArgument
	}
	digest := sha256.Sum256(message)
	signature, err := signer.Sign(ctx, digest[:])
	if err != nil {
		return SignedCheckpoint{}, ErrUnavailable
	}
	result := SignedCheckpoint{
		Checkpoint: body,
		Digest:     "ac1_" + hex.EncodeToString(digest[:]),
		Signature:  append([]byte(nil), signature...),
	}
	if err := ValidateCheckpoint(result); err != nil {
		return SignedCheckpoint{}, err
	}
	return result, nil
}

func ValidateCheckpoint(checkpoint SignedCheckpoint) error {
	if checkpoint.Version != ContractVersion || checkpoint.SchemaVersion != SchemaVersion ||
		!ValidRealmID(checkpoint.RealmID) || checkpoint.AuthorityEpoch == 0 ||
		checkpoint.AuthoritySequence == 0 || len(checkpoint.AuthorityPublicKey) != ed25519.PublicKeySize ||
		!digestPattern.MatchString(checkpoint.AuditHead) || !canonicalSecond(checkpoint.CreatedAt) ||
		len(checkpoint.Signature) != ed25519.SignatureSize {
		return ErrCorruptState
	}
	if checkpoint.AuthoritySequence == 1 {
		if checkpoint.PreviousDigest != "" {
			return ErrCorruptState
		}
	} else if !digestPattern.MatchString(checkpoint.PreviousDigest) {
		return ErrCorruptState
	}
	principal, err := identityprincipal.FromEd25519PublicKey(ed25519.PublicKey(checkpoint.AuthorityPublicKey))
	if err != nil || principal.String() != checkpoint.AuthorityPrincipal {
		return ErrCorruptState
	}
	message, err := checkpointMessage(checkpoint.Checkpoint)
	if err != nil {
		return ErrCorruptState
	}
	digest := sha256.Sum256(message)
	if checkpoint.Digest != "ac1_"+hex.EncodeToString(digest[:]) ||
		!ed25519.Verify(ed25519.PublicKey(checkpoint.AuthorityPublicKey), digest[:], checkpoint.Signature) {
		return ErrCorruptState
	}
	return nil
}

func checkpointsEqual(left, right SignedCheckpoint) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftRaw, rightRaw)
}
