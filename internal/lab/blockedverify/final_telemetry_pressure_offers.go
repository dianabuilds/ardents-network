package blockedverify

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

func validFinalAdmissionOffers(value finalAdmissionInput, cellSeed, label string) (uint16, uint16, uint32, bool) {
	seed, err := hex.DecodeString(cellSeed)
	if err != nil || len(seed) != 32 || len(value.Outcomes) != 100 {
		return 0, 0, 0, false
	}
	derived := sha256.Sum256(append(append(append([]byte(nil), seed...), 0), label...))
	seen := make(map[string]bool, len(value.Outcomes))
	var refused uint16
	var maximum uint32
	for index, outcome := range value.Outcomes {
		ordinal := make([]byte, 8)
		binary.BigEndian.PutUint64(ordinal, uint64(index))
		canary := sha256.Sum256(append(append([]byte(nil), derived[:]...), ordinal...))
		digest := sha256.Sum256(canary[:])
		wantCanary := hex.EncodeToString(digest[:])
		wantOffset := uint32(index * 100)
		if outcome.Ordinal != uint16(index) || outcome.CanarySHA256 != wantCanary || seen[wantCanary] ||
			outcome.ScheduledOffsetMillis != wantOffset || outcome.StartedOffsetMillis < wantOffset ||
			outcome.StartedOffsetMillis > wantOffset+250 || !outcome.Refused || outcome.RefusalMillis >= 1_000 {
			return 0, 0, 0, false
		}
		seen[wantCanary] = true
		refused++
		maximum = max(maximum, outcome.RefusalMillis)
	}
	return uint16(len(value.Outcomes)), refused, maximum,
		value.Offers == uint16(len(value.Outcomes)) && value.Refused == refused && value.MaximumMillis == maximum
}
