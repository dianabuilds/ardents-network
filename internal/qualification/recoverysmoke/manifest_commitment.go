package recoverysmoke

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

func publicManifestDigest(value recovery.PublicManifest) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-h3-recovery-public-manifest-v1\x00"))
	for _, field := range [][32]byte{value.RouteManifest, value.NetworkID, value.AuthorityPublic,
		value.IntroductionPublic, value.Target, value.InstancePublic, value.ClientPrincipal, value.PublisherPrincipal} {
		_, _ = hash.Write(field[:])
	}
	_, _ = hash.Write(value.CredentialSignature[:])
	var numbers [52]byte
	binary.BigEndian.PutUint64(numbers[0:8], value.CredentialGeneration)
	binary.BigEndian.PutUint64(numbers[8:16], uint64(value.CredentialNotBefore))
	binary.BigEndian.PutUint64(numbers[16:24], uint64(value.CredentialNotAfter))
	binary.BigEndian.PutUint32(numbers[24:28], value.CredentialCapabilities)
	binary.BigEndian.PutUint64(numbers[28:36], uint64(value.WorkSafetyNotAfter))
	binary.BigEndian.PutUint64(numbers[36:44], uint64(value.WorkSafetyMaximum))
	binary.BigEndian.PutUint64(numbers[44:52], uint64(value.NoNewRecoveryAfter))
	_, _ = hash.Write(numbers[:])
	_, _ = hash.Write([]byte(value.RouteProfile))
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}
