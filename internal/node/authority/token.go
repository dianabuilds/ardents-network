package authority

import (
	"crypto/sha256"
	"errors"
	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// TokenVerifier authenticates a selected-profile role token for
// forwarding and control duties. Host reservation and spend stay with their
// respective duty owners.
func (source Source) TokenVerifier(receiver route.ClosedRoleReceiver, clock func() time.Time) route.ClosedAdmissionVerifier {
	return func(input route.ClosedAdmissionVerification) (route.ClosedAdmissionApproval, error) {
		if input.Class < 1 || input.Class > 3 || source.CurrentProfile == nil {
			return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
		}
		keyID, framed := admissiontoken.ClosedTokenKeyID(input.Token)
		if !framed {
			return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
		}
		profile, available := source.CurrentProfile()
		now := clock().UTC()
		if !available || profile.NetworkID != receiver.NetworkID || profile.StateGeneration != receiver.StateGeneration || profile.StateDigest != receiver.StateDigest ||
			profile.Digest != receiver.ProfileDigest || profile.NotBefore.After(now) || !now.Before(profile.NotAfter) {
			return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
		}
		for index := uint8(0); index < profile.TokenKeyCount; index++ {
			key := profile.TokenKeys[index]
			if key.Class != input.Class || key.WindowStart != now.Truncate(time.Hour) || sha256.Sum256(key.SPKI[:]) != keyID {
				continue
			}
			context := admissiontoken.ClosedTokenContext{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID,
				IssuerNodeID: profile.IssuerNodeID, ReceiverDutyGeneration: receiver.DutyGeneration, Class: input.Class, WindowStart: key.WindowStart}
			if admissiontoken.VerifyClosedToken(context, key.SPKI[:], input.Token) == nil {
				return route.ClosedAdmissionApproval{Window: key.WindowStart}, nil
			}
		}
		return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
	}
}
