package recovery

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type hostScopeEvidence struct {
	Adapter, AdapterProjection       string
	Machine, Campaign, Source, Image [32]byte
	Commitment                       [32]byte
}

func decodeHostScope(raw []byte) (hostScopeEvidence, error) {
	var value hostScopeEvidence
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) == 0 || len(raw) > 64<<10 {
		return value, errors.New("host-observation scope is malformed")
	}
	if err := decoder.Decode(&value); err != nil {
		return value, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return value, errors.New("host-observation scope contains multiple values")
	}
	canonical, err := json.Marshal(value)
	if err != nil || !canonicalJSONEqual(raw, canonical) {
		return value, errors.Join(err, errors.New("host-observation scope is not canonical"))
	}
	return value, nil
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
