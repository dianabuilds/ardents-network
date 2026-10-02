package issuerprofile

import (
	"crypto/sha256"
	"math"
	"time"
)

// Key is one exact public class/hour key, never a signing key.
type Key struct {
	Window uint64
	Class  uint8
	SPKI   []byte
}

// ValidateCohorts checks the complete ordered inventory for one selected interval.
// Every class/hour has one distinct canonical public key; this conveys no quota
// or State authority.
func ValidateCohorts(startTime, endTime time.Time, keys []Key) error {
	start, end := startTime.Unix(), endTime.Unix()
	if start < 0 || start > math.MaxInt64-21600 || start%3600 != 0 || end <= start || end-start > 21600 || end%3600 != 0 || startTime.Nanosecond() != 0 || endTime.Nanosecond() != 0 || len(keys) != int((end-start)/3600)*3 {
		return ErrInvalid
	}
	seen := map[[32]byte]bool{}
	for i, key := range keys {
		if key.Window != uint64(start)+uint64(i/3)*3600 || key.Class != uint8(i%3+1) {
			return ErrInvalid
		}
		if _, ok := ParseKey(key.SPKI); !ok {
			return ErrInvalid
		}
		// A dedicated key cannot be reused across class/hour cohorts.
		id := sha256.Sum256(key.SPKI)
		if seen[id] {
			return ErrInvalid
		}
		seen[id] = true
	}
	return nil
}

func cloneKeys(keys []Key) []Key {
	result := make([]Key, len(keys))
	for i, k := range keys {
		result[i] = k
		result[i].SPKI = append([]byte(nil), k.SPKI...)
	}
	return result
}
