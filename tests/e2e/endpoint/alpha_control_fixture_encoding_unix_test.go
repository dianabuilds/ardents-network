//go:build linux

package endpoint_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"sort"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
)

func signAlphaCatalogFixture(input alphacontrol.Catalog, signer ed25519.PrivateKey) ([]byte, error) {
	return signAlphaCatalogFields("ACA1", "ardents-alpha-control-catalog-v1\x00", input.Cohort, input.Generation,
		input.NotBefore.Unix(), input.NotAfter.Unix(), input.PreviousDigest, input.Components[:], signer), nil
}

func signAlphaCatalogV2Fixture(input alphacontrol.CatalogV2, signer ed25519.PrivateKey) ([]byte, error) {
	return signAlphaCatalogFields("ACA2", "ardents-alpha-control-catalog-v2\x00", input.Cohort, input.Generation,
		input.NotBefore.Unix(), input.NotAfter.Unix(), input.PreviousDigest, input.Components[:], signer), nil
}

func signAlphaCatalogFields(magic, domain, cohort string, generation uint64, notBefore, notAfter int64, previous [32]byte,
	components []alphacontrol.Component, signer ed25519.PrivateKey,
) []byte {
	payload := append([]byte(magic), byte(magic[3]-'0'), byte(len(cohort)))
	payload = append(payload, cohort...)
	payload = binary.BigEndian.AppendUint64(payload, generation)
	payload = binary.BigEndian.AppendUint64(payload, uint64(notBefore))
	payload = binary.BigEndian.AppendUint64(payload, uint64(notAfter))
	payload = append(payload, previous[:]...)
	payload = append(payload, byte(len(components)))
	for _, component := range components {
		payload = append(payload, byte(component.Class))
		payload = append(payload, component.RootID[:]...)
		payload = binary.BigEndian.AppendUint64(payload, component.Generation)
		payload = binary.BigEndian.AppendUint64(payload, uint64(component.NotAfter.Unix()))
		payload = binary.BigEndian.AppendUint32(payload, component.Size)
		payload = append(payload, component.Digest[:]...)
	}
	return append(payload, ed25519.Sign(signer, append([]byte(domain), payload...))...)
}

func signAlphaComponentFixture(input alphacontrol.ComponentStatement, signer ed25519.PrivateKey) ([]byte, error) {
	payload := append([]byte("ACS1"), 1, byte(input.Class))
	payload = binary.BigEndian.AppendUint64(payload, input.Generation)
	payload = binary.BigEndian.AppendUint64(payload, uint64(input.NotBefore.Unix()))
	payload = binary.BigEndian.AppendUint64(payload, uint64(input.NotAfter.Unix()))
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(input.Body)))
	payload = append(payload, input.Body...)
	return append(payload, ed25519.Sign(signer, append([]byte("ardents-alpha-control-component-v1\x00"), payload...))...), nil
}

func encodeReleaseEvidenceFixture(value inspection.ReleaseEvidence) ([]byte, error) {
	result := append([]byte("ACR1"), 1)
	result = append(result, value.ArtifactDigest[:]...)
	for _, text := range []string{value.TargetPath, value.ReleaseIdentity, value.BuildIdentity, value.ProtocolPhase, value.BuildState} {
		result = appendAlphaTextFixture(result, text)
	}
	return result, nil
}

func encodeNetworkEvidenceFixture(value inspection.NetworkEvidence) ([]byte, error) {
	authorities := append([]ed25519.PublicKey(nil), value.Authorities...)
	sort.Slice(authorities, func(left, right int) bool {
		leftID, rightID := sha256.Sum256(authorities[left]), sha256.Sum256(authorities[right])
		return bytes.Compare(leftID[:], rightID[:]) < 0
	})
	result := append([]byte("ACN1"), 1)
	result = append(result, value.NetworkID[:]...)
	result = append(result, value.EpochDigest[:]...)
	result = appendAlphaTextFixture(result, value.Profile)
	result = append(result, value.Threshold, byte(len(authorities)))
	for _, authority := range authorities {
		result = append(result, authority...)
	}
	result = appendAlphaBytesFixture(result, value.Epoch)
	for _, group := range [][][]byte{value.Inputs, value.Materials} {
		result = append(result, byte(len(group)))
		for _, member := range group {
			result = appendAlphaBytesFixture(result, member)
		}
	}
	return result, nil
}

func encodeCompatibilityEvidenceFixture(value inspection.CompatibilityEvidence) ([]byte, error) {
	result := append([]byte("ACC1"), 1)
	result = append(result, value.ReleaseDigest[:]...)
	result = appendAlphaTextFixture(result, value.ReleaseBuildIdentity)
	result = appendAlphaTextFixture(result, value.ProtocolPhase)
	result = append(result, value.NetworkDigest[:]...)
	result = binary.BigEndian.AppendUint64(result, value.NetworkEpoch)
	return appendAlphaTextFixture(result, value.NetworkProfile), nil
}

func appendAlphaTextFixture(result []byte, value string) []byte {
	return append(append(result, byte(len(value))), value...)
}

func appendAlphaBytesFixture(result, value []byte) []byte {
	result = binary.BigEndian.AppendUint32(result, uint32(len(value)))
	return append(result, value...)
}
