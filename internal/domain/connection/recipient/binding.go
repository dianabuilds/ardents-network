package recipient

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
	"github.com/dianabuilds/ardents-network/internal/domain/publication"
)

// ErrBinding identifies a recipient whose immutable facts or safety bounds differ.
var ErrBinding = errors.New("recipient immutable binding or safety bounds differ")

// Facts holds the immutable destination, transcript commitments, and candidate bounds.
type Facts struct {
	Identity   wire.LogicalIdentity
	Logical    [32]byte
	Attachment [32]byte
	Deadline   time.Time
	Revision   uint64
}

// Derive checks the logical facts against verified Publication and derives the
// exact transcript commitments for this candidate Attachment.
func Derive(proof publication.Proof, identity wire.LogicalIdentity, plaintextDigest [32]byte,
	deadline, localEnd, now time.Time, generation, revision uint64) (Facts, error) {
	value := proof.Delegation()
	if proof.Digest() == [32]byte{} || value.Generation == 0 || value.Instance == [32]byte{} ||
		identity.Profile == [32]byte{} || plaintextDigest == [32]byte{} || now.Before(value.NotBefore) || !now.Before(value.NotAfter) ||
		identity.Network != value.Network || identity.Target != value.Target || identity.Publication != proof.Digest() ||
		identity.ConnectionNonce == [32]byte{} || identity.InitiatorBinding == [32]byte{} || revision == 0 || generation == 0 ||
		deadline != deadline.UTC().Truncate(time.Second) || !now.Before(deadline) || deadline.After(now.Add(30*time.Second)) ||
		identity.WorkSafetyNotAfter <= now.Unix() || identity.WorkSafetyMaximum < identity.WorkSafetyNotAfter ||
		identity.NoNewRecoveryAfter <= 0 || identity.NoNewRecoveryAfter > identity.WorkSafetyNotAfter || deadline.Unix() > identity.WorkSafetyNotAfter ||
		localEnd.IsZero() || time.Unix(identity.WorkSafetyMaximum, 0).After(localEnd) || time.Unix(identity.WorkSafetyMaximum, 0).After(value.NotAfter) {
		return Facts{}, ErrBinding
	}
	identity.Instance = value.Instance
	identity.InstanceGeneration = value.Generation
	logical, attachment, err := wire.Contexts(identity, plaintextDigest, generation)
	if err != nil {
		return Facts{}, err
	}
	return Facts{Identity: identity, Logical: logical, Attachment: attachment, Deadline: deadline, Revision: revision}, nil
}
