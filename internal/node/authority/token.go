package authority

import (
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// TokenVerifier authenticates a selected-profile role token for
// forwarding and control duties. Host reservation and spend stay with their
// respective duty owners.
func (source Source) TokenVerifier(receiver route.ClosedRoleReceiver, clock func() time.Time) route.ClosedAdmissionVerifier {
	return func(input route.ClosedAdmissionVerification) (route.ClosedAdmissionApproval, error) {
		if len(input.Token) != 354 || input.Class < 1 || input.Class > 3 || source.CurrentProfile == nil {
			return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
		}
		profile, available := source.CurrentProfile()
		now := clock().UTC()
		if !available || profile.NetworkID != receiver.NetworkID || profile.StateGeneration != receiver.StateGeneration || profile.StateDigest != receiver.StateDigest ||
			profile.Digest != receiver.ProfileDigest || profile.NotBefore.After(now) || !now.Before(profile.NotAfter) {
			return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
		}
		var keyID [32]byte
		copy(keyID[:], input.Token[66:98])
		for index := uint8(0); index < profile.TokenKeyCount; index++ {
			key := profile.TokenKeys[index]
			if key.Class != input.Class || key.WindowStart != now.Truncate(time.Hour) || sha256.Sum256(key.SPKI[:]) != keyID {
				continue
			}
			context := credential.ClosedTokenContext{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID,
				IssuerNodeID: profile.IssuerNodeID, ReceiverDutyGeneration: receiver.DutyGeneration, Class: input.Class, WindowStart: key.WindowStart}
			if credential.VerifyClosedToken(context, key.SPKI[:], input.Token) == nil {
				return route.ClosedAdmissionApproval{Window: key.WindowStart}, nil
			}
		}
		return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
	}
}
