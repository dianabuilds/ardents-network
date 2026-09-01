package inspection

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"
)

const maximumEvidenceBytes = 8 << 20

// ReleaseEvidence binds a separately signed release statement to the exact
// independently verified Release Decision identity.
type ReleaseEvidence struct {
	ArtifactDigest                             [32]byte
	TargetPath, ReleaseIdentity, BuildIdentity string
	ProtocolPhase, BuildState                  string
}

// NetworkEvidence is the complete bounded offline Network State decision and
// its disclosed authority configuration. The statement signer, not the
// catalog, is the authority for disclosing this configuration.
type NetworkEvidence struct {
	NetworkID   [32]byte
	EpochDigest [32]byte
	Profile     string
	Threshold   uint8
	Authorities []ed25519.PublicKey
	Epoch       []byte
	Inputs      [][]byte
	Materials   [][]byte
}

// CompatibilityEvidence binds compatibility to independently evaluated
// Release and Network State identities.
type CompatibilityEvidence struct {
	ReleaseDigest        [32]byte
	ReleaseBuildIdentity string
	ProtocolPhase        string
	NetworkDigest        [32]byte
	NetworkEpoch         uint64
	NetworkProfile       string
}

func encodeReleaseEvidence(value ReleaseEvidence) ([]byte, error) {
	if value.ArtifactDigest == [32]byte{} || !validText(value.TargetPath, 255) || !validText(value.ReleaseIdentity, 255) ||
		!validText(value.BuildIdentity, 255) || !validText(value.ProtocolPhase, 64) || !validText(value.BuildState, 64) {
		return nil, errors.New("alpha release evidence is invalid")
	}
	result := append([]byte("ACR1"), 1)
	result = append(result, value.ArtifactDigest[:]...)
	for _, text := range []string{value.TargetPath, value.ReleaseIdentity, value.BuildIdentity, value.ProtocolPhase, value.BuildState} {
		result = appendText(result, text)
	}
	return result, nil
}

func decodeReleaseEvidence(raw []byte) (ReleaseEvidence, error) {
	if len(raw) < 37 || string(raw[:4]) != "ACR1" || raw[4] != 1 {
		return ReleaseEvidence{}, errors.New("alpha release evidence version is invalid")
	}
	value := ReleaseEvidence{}
	copy(value.ArtifactDigest[:], raw[5:37])
	offset := 37
	values := []*string{&value.TargetPath, &value.ReleaseIdentity, &value.BuildIdentity, &value.ProtocolPhase, &value.BuildState}
	for _, field := range values {
		text, next, err := readText(raw, offset, 255)
		if err != nil {
			return ReleaseEvidence{}, err
		}
		*field, offset = text, next
	}
	if offset != len(raw) {
		return ReleaseEvidence{}, errors.New("alpha release evidence has trailing bytes")
	}
	_, err := encodeReleaseEvidence(value)
	return value, err
}

func encodeNetworkEvidence(value NetworkEvidence) ([]byte, error) {
	if value.NetworkID == [32]byte{} || value.EpochDigest == [32]byte{} || !validText(value.Profile, 64) || value.Threshold == 0 ||
		int(value.Threshold) > len(value.Authorities) || len(value.Authorities) > 16 || len(value.Epoch) == 0 || len(value.Epoch) > 1<<20 ||
		len(value.Inputs) > 64 || len(value.Materials) > 64 {
		return nil, errors.New("alpha network evidence is invalid")
	}
	authorities := append([]ed25519.PublicKey(nil), value.Authorities...)
	sort.Slice(authorities, func(left, right int) bool {
		leftID, rightID := sha256Digest(authorities[left]), sha256Digest(authorities[right])
		return bytes.Compare(leftID[:], rightID[:]) < 0
	})
	result := append([]byte("ACN1"), 1)
	result = append(result, value.NetworkID[:]...)
	result = append(result, value.EpochDigest[:]...)
	result = appendText(result, value.Profile)
	result = append(result, value.Threshold, byte(len(authorities)))
	var previous [32]byte
	for index, authority := range authorities {
		if len(authority) != ed25519.PublicKeySize {
			return nil, errors.New("alpha network authority is invalid")
		}
		id := sha256Digest(authority)
		if index != 0 && id == previous {
			return nil, errors.New("alpha network authority is duplicated")
		}
		previous = id
		result = append(result, authority...)
	}
	result = appendBytes(result, value.Epoch)
	for _, group := range [][][]byte{value.Inputs, value.Materials} {
		result = append(result, byte(len(group)))
		for _, member := range group {
			if len(member) == 0 || len(member) > 35<<10 {
				return nil, errors.New("alpha network evidence member is invalid")
			}
			result = appendBytes(result, member)
		}
	}
	if len(result) > maximumEvidenceBytes {
		return nil, errors.New("alpha network evidence exceeds its bound")
	}
	return result, nil
}

func decodeNetworkEvidence(raw []byte) (NetworkEvidence, error) {
	if len(raw) < 4+1+32+32+1+2+4 || len(raw) > maximumEvidenceBytes || string(raw[:4]) != "ACN1" || raw[4] != 1 {
		return NetworkEvidence{}, errors.New("alpha network evidence version is invalid")
	}
	value := NetworkEvidence{}
	copy(value.NetworkID[:], raw[5:37])
	copy(value.EpochDigest[:], raw[37:69])
	profile, offset, err := readText(raw, 69, 64)
	if err != nil || offset+2 > len(raw) {
		return NetworkEvidence{}, errors.New("alpha network evidence profile is invalid")
	}
	value.Profile, value.Threshold = profile, raw[offset]
	count := int(raw[offset+1])
	offset += 2
	if count == 0 || count > 16 || offset+count*ed25519.PublicKeySize > len(raw) {
		return NetworkEvidence{}, errors.New("alpha network evidence authority count is invalid")
	}
	var previous [32]byte
	for index := 0; index < count; index++ {
		public := append(ed25519.PublicKey(nil), raw[offset:offset+ed25519.PublicKeySize]...)
		offset += ed25519.PublicKeySize
		id := sha256Digest(public)
		if index != 0 && bytes.Compare(previous[:], id[:]) >= 0 {
			return NetworkEvidence{}, errors.New("alpha network evidence authorities are not canonical")
		}
		previous = id
		value.Authorities = append(value.Authorities, public)
	}
	value.Epoch, offset, err = readBytes(raw, offset, 1<<20)
	if err != nil {
		return NetworkEvidence{}, err
	}
	groups := []*[][]byte{&value.Inputs, &value.Materials}
	for _, group := range groups {
		if offset >= len(raw) || raw[offset] > 64 {
			return NetworkEvidence{}, errors.New("alpha network evidence member count is invalid")
		}
		count := int(raw[offset])
		offset++
		for range count {
			member, next, readErr := readBytes(raw, offset, 35<<10)
			if readErr != nil {
				return NetworkEvidence{}, readErr
			}
			*group, offset = append(*group, member), next
		}
	}
	if offset != len(raw) {
		return NetworkEvidence{}, errors.New("alpha network evidence has trailing bytes")
	}
	_, err = encodeNetworkEvidence(value)
	return value, err
}

func encodeCompatibilityEvidence(value CompatibilityEvidence) ([]byte, error) {
	if value.ReleaseDigest == [32]byte{} || value.NetworkDigest == [32]byte{} || value.NetworkEpoch == 0 ||
		!validText(value.ReleaseBuildIdentity, 255) || !validText(value.ProtocolPhase, 64) || !validText(value.NetworkProfile, 64) {
		return nil, errors.New("alpha compatibility evidence is invalid")
	}
	result := append([]byte("ACC1"), 1)
	result = append(result, value.ReleaseDigest[:]...)
	result = appendText(result, value.ReleaseBuildIdentity)
	result = appendText(result, value.ProtocolPhase)
	result = append(result, value.NetworkDigest[:]...)
	result = binary.BigEndian.AppendUint64(result, value.NetworkEpoch)
	return appendText(result, value.NetworkProfile), nil
}

func decodeCompatibilityEvidence(raw []byte) (CompatibilityEvidence, error) {
	if len(raw) < 4+1+32+1+1+32+8+1 || string(raw[:4]) != "ACC1" || raw[4] != 1 {
		return CompatibilityEvidence{}, errors.New("alpha compatibility evidence version is invalid")
	}
	value := CompatibilityEvidence{}
	copy(value.ReleaseDigest[:], raw[5:37])
	build, offset, err := readText(raw, 37, 255)
	if err != nil {
		return CompatibilityEvidence{}, err
	}
	phase, offset, err := readText(raw, offset, 64)
	if err != nil || offset+32+8 > len(raw) {
		return CompatibilityEvidence{}, errors.New("alpha compatibility evidence release binding is invalid")
	}
	value.ReleaseBuildIdentity, value.ProtocolPhase = build, phase
	copy(value.NetworkDigest[:], raw[offset:offset+32])
	offset += 32
	value.NetworkEpoch = binary.BigEndian.Uint64(raw[offset : offset+8])
	offset += 8
	profile, offset, err := readText(raw, offset, 64)
	if err != nil || offset != len(raw) {
		return CompatibilityEvidence{}, errors.New("alpha compatibility evidence network binding is invalid")
	}
	value.NetworkProfile = profile
	_, err = encodeCompatibilityEvidence(value)
	return value, err
}

func sha256Digest(value []byte) [32]byte { return sha256.Sum256(value) }

func validText(value string, maximum int) bool { return len(value) > 0 && len(value) <= maximum }

func appendText(result []byte, value string) []byte {
	result = append(result, byte(len(value)))
	return append(result, value...)
}

func readText(raw []byte, offset, maximum int) (string, int, error) {
	if offset >= len(raw) || int(raw[offset]) == 0 || int(raw[offset]) > maximum || offset+1+int(raw[offset]) > len(raw) {
		return "", 0, errors.New("alpha evidence text is invalid")
	}
	length := int(raw[offset])
	return string(raw[offset+1 : offset+1+length]), offset + 1 + length, nil
}

func appendBytes(result, value []byte) []byte {
	result = binary.BigEndian.AppendUint32(result, uint32(len(value)))
	return append(result, value...)
}

func readBytes(raw []byte, offset, maximum int) ([]byte, int, error) {
	if offset+4 > len(raw) {
		return nil, 0, errors.New("alpha evidence bytes are truncated")
	}
	length := int(binary.BigEndian.Uint32(raw[offset : offset+4]))
	offset += 4
	if length == 0 || length > maximum || offset+length > len(raw) {
		return nil, 0, errors.New("alpha evidence bytes are invalid")
	}
	return append([]byte(nil), raw[offset:offset+length]...), offset + length, nil
}
