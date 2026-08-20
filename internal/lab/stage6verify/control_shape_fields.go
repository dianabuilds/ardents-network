package stage6verify

import (
	"crypto/sha256"
	"encoding/json"
)

func controlRecord(records []decodedRecord, name string) (decodedRecord, bool) {
	for _, record := range records {
		if record.Name == name {
			return record, true
		}
	}
	return decodedRecord{}, false
}

func verifiedControlDigest(value controlOperationEvidence) [32]byte {
	value.OperationDigest = [32]byte{}
	value.Network, value.Nonce, value.Deadline = [32]byte{}, [32]byte{}, 0
	raw, _ := json.Marshal(value)
	return sha256.Sum256(append([]byte("ardents-name-control-operation-v1\x00"), raw...))
}

func controlSurface(kind string) string {
	if kind == "claim" {
		return "root-claim"
	}
	if kind == "policy" || kind == "recovery" {
		return "policy-recovery"
	}
	return "renewal-update"
}

func controlWorkBits(kind string) uint8 {
	if kind == "claim" {
		return 18
	}
	if kind == "policy" || kind == "recovery" {
		return 17
	}
	return 16
}
