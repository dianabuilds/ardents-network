package recoverysmoke

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

func recoveryCellManifest(direction string, seed [32]byte, planned uint32) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-h3-recovery-cell-manifest-v1\x00" + direction +
		"\x00carrier-channel\x00carrier-attachment-deadline=13s\x00chunk-delay=30ms\x00"))
	_, _ = hash.Write(seed[:])
	var values [12]byte
	binary.BigEndian.PutUint32(values[:4], recoveryBytes)
	binary.BigEndian.PutUint32(values[4:8], planned)
	binary.BigEndian.PutUint32(values[8:12], 32)
	_, _ = hash.Write(values[:])
	return hex.EncodeToString(hash.Sum(nil))
}
