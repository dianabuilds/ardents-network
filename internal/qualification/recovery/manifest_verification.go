package recovery

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

func verifyManifest(value Evidence) Result {
	manifest := value.Manifest
	manifestDigest := publicManifestDigest(manifest)
	if hex.EncodeToString(manifestDigest[:]) != value.ManifestDigest {
		return invalid("public manifest does not match its commitment")
	}
	derived := sha256.Sum256(append([]byte("ardents-h3-service-target-v1\x00"), manifest.AuthorityPublic[:]...))
	if derived != manifest.Target || value.Target != manifest.Target || value.Instance != manifest.InstancePublic ||
		value.NetworkID != manifest.NetworkID || value.CandidateView != manifest.RouteManifest ||
		value.AuthorityPublic != manifest.AuthorityPublic || value.ClientPrincipal != manifest.ClientPrincipal ||
		value.PublisherPrincipal != manifest.PublisherPrincipal || value.RouteProfile != manifest.RouteProfile ||
		value.CredentialGeneration != manifest.CredentialGeneration || value.CredentialNotBefore != manifest.CredentialNotBefore ||
		value.CredentialNotAfter != manifest.CredentialNotAfter || value.WorkSafetyNotAfter != manifest.WorkSafetyNotAfter ||
		value.WorkSafetyMaximum != manifest.WorkSafetyMaximum || value.NoNewRecoveryAfter != manifest.NoNewRecoveryAfter {
		return invalid("public connection binding differs from its manifest")
	}
	if !ed25519.Verify(ed25519.PublicKey(manifest.AuthorityPublic[:]), credentialBody(manifest), manifest.CredentialSignature[:]) {
		return fail("active Service Credential signature is invalid")
	}
	expectedIsolation := sha256.Sum256(append([]byte("isolation\x00"), manifestDigest[:]...))
	expectedDestination := sha256.Sum256(append([]byte("destination\x00"), manifest.Target[:]...))
	if value.IsolationContext != expectedIsolation || value.DestinationBinding != expectedDestination {
		return invalid("context or Destination Binding is not canonically derived")
	}
	return Result{Verdict: "pass"}
}

func publicManifestDigest(value PublicManifest) [32]byte {
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

func credentialBody(value PublicManifest) []byte {
	encoded := make([]byte, 161)
	copy(encoded[:5], []byte{'A', 'S', 'C', 'R', 1})
	offset := 5
	for _, field := range [][32]byte{value.AuthorityPublic, value.Target, value.InstancePublic} {
		copy(encoded[offset:offset+32], field[:])
		offset += 32
	}
	binary.BigEndian.PutUint64(encoded[offset:offset+8], value.CredentialGeneration)
	offset += 8
	binary.BigEndian.PutUint64(encoded[offset:offset+8], uint64(value.CredentialNotBefore))
	offset += 8
	binary.BigEndian.PutUint64(encoded[offset:offset+8], uint64(value.CredentialNotAfter))
	offset += 8
	copy(encoded[offset:offset+32], value.NetworkID[:])
	offset += 32
	binary.BigEndian.PutUint32(encoded[offset:offset+4], value.CredentialCapabilities)
	return encoded
}

func hexDigest(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
