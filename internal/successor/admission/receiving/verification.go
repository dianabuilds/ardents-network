package receiving

import (
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
)

// Receiver is the public authority binding of the duty accepting a token.
// Channel purpose and TLS framing remain with the protocol adapter.
type Receiver struct {
	NetworkID, StateGeneration, StateDigest, ProfileDigest, NodeID [32]byte
	DutyGeneration                                                 uint64
}

// VerifyToken binds a token to this receiver and the current State-supplied
// profile, then returns its verified spend hour. The profile must come from a
// live authority owner; signature bytes alone do not establish currentness.
func VerifyToken(profile admission.AuthorityFacts, receiver Receiver, class admission.Class, raw []byte, now time.Time) (time.Time, error) {
	unavailable := errors.New("admission token is unavailable")
	if class.Lifetime() == 0 || now.IsZero() || int(profile.TokenKeyCount) > len(profile.TokenKeys) {
		return time.Time{}, unavailable
	}
	keyID, framed := token.ClosedTokenKeyID(raw)
	if !framed {
		return time.Time{}, unavailable
	}
	now = now.UTC()
	if profile.NetworkID != receiver.NetworkID || profile.StateGeneration != receiver.StateGeneration || profile.StateDigest != receiver.StateDigest ||
		profile.Digest != receiver.ProfileDigest || profile.NotBefore.After(now) || !now.Before(profile.NotAfter) {
		return time.Time{}, unavailable
	}
	for _, key := range profile.TokenKeys[:profile.TokenKeyCount] {
		if key.Class != uint8(class) || key.WindowStart != now.Truncate(time.Hour) || sha256.Sum256(key.SPKI[:]) != keyID {
			continue
		}
		context := token.ClosedTokenContext{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID,
			IssuerNodeID: profile.IssuerNodeID, ReceiverDutyGeneration: receiver.DutyGeneration, Class: uint8(class), WindowStart: key.WindowStart}
		if token.VerifyClosedToken(context, key.SPKI[:], raw) == nil {
			return key.WindowStart, nil
		}
	}
	return time.Time{}, unavailable
}
