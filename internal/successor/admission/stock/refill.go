package stock

import "errors"

// RefillPlan returns copied receiver intent for the exact retained refill
// or the bounded Control stock needed before other issuance. The application
// supplies whether that issuance consumes a Control token and authorizes delivery.
func (owner *Owner) RefillPlan(requested [][32]byte, class uint8, requiresAdmissionToken bool) ([][32]byte, error) {
	if owner == nil {
		return nil, errors.New("holder unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.current()
	permission := owner.permission
	if err != nil || !permission.CurrentFor(profile, now) {
		return nil, errors.New("text issuer stock owner unavailable")
	}
	if batch := permission.pending; batch != nil {
		if !batch.Refill {
			return nil, nil
		}
		receivers := make([][32]byte, len(batch.Challenges))
		for index, challenge := range batch.Challenges {
			if challenge.Class != 1 || challenge.ReceiverNodeID != profile.IssuerNodeID {
				return nil, errors.New("text issuer pending stock binding unavailable")
			}
			receivers[index] = challenge.ReceiverNodeID
		}
		return receivers, nil
	}
	self := class == 1 && len(requested) != 0
	for _, receiver := range requested {
		self = self && receiver == profile.IssuerNodeID
	}
	if self {
		return nil, nil
	}
	ready := permission.StockCountForDuty(profile.Digest, profile.IssuerNodeID, profile.IssuerDutyGeneration, 1)
	remaining := permission.Remaining(1)
	if ready >= 2 || remaining == 0 || requiresAdmissionToken && (ready == 0 || remaining < 2) {
		return nil, nil
	}
	receivers := make([][32]byte, min(remaining, 32))
	for index := range receivers {
		receivers[index] = profile.IssuerNodeID
	}
	return receivers, nil
}
