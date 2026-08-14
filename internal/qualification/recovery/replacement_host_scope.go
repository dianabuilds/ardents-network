package recovery

import (
	"crypto/sha256"
	"strings"
)

type hostScopeEvidence struct {
	Adapter, AdapterProjection       string
	Machine, Campaign, Source, Image [32]byte
	Commitment                       [32]byte
}

func hostScopeCommitment(value hostScopeEvidence) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-qualification-host-scope-v1\x00" + value.Adapter + "\x00"))
	for _, field := range [][32]byte{value.Machine, value.Campaign, value.Source, value.Image} {
		_, _ = hash.Write(field[:])
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func validHostScope(value hostScopeEvidence, sourceCommit, imageID string) bool {
	return value.Adapter != "" && value.Machine != [32]byte{} && value.Campaign != [32]byte{} &&
		value.Source == sha256.Sum256([]byte(sourceCommit)) && value.Image == sha256.Sum256([]byte(imageID)) &&
		value.Commitment != [32]byte{} && value.Commitment == hostScopeCommitment(value)
}

func validDockerHostScopeProjection(value hostScopeEvidence, manifestDigest string) bool {
	return value.Adapter == "docker-compose-v1" && strings.HasPrefix(value.AdapterProjection, "ardents-recovery-") &&
		value.Campaign == sha256.Sum256([]byte(value.AdapterProjection+"\x00"+manifestDigest))
}
