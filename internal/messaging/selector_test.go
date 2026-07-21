package messaging

import (
	"bytes"
	"encoding/json"
	"testing"

	identityapi "ardents/internal/identity"

	"github.com/stretchr/testify/require"
)

func TestDeriveSelectorDeterministicVector(t *testing.T) {
	resolved := fixedResolvedCapability(t, 7)
	material, err := Derive(resolved)
	require.NoError(t, err)
	require.Equal(t, "/ardents/1/kaciboigy5ukazbvbf2ohtalvbpupr6k/proto", material.ContentTopic)
	require.Equal(t, "f7592a38caf0e765d4707203a93c1af07fcd3e4ddde3c421cb5451d1e56d1971", stringHex(material.EnvelopeKey()))
}

func TestDeriveSelectorInteroperatesAndRotates(t *testing.T) {
	first := fixedResolvedCapability(t, 7)
	peerCopy := first
	left, err := Derive(first)
	require.NoError(t, err)
	right, err := Derive(peerCopy)
	require.NoError(t, err)
	require.Equal(t, left.ContentTopic, right.ContentTopic)
	require.Equal(t, left.EnvelopeKey(), right.EnvelopeKey())

	peerCopy.Generation++
	rotated, err := Derive(peerCopy)
	require.NoError(t, err)
	require.NotEqual(t, left.ContentTopic, rotated.ContentTopic)
	require.NotEqual(t, left.EnvelopeKey(), rotated.EnvelopeKey())
}

func TestFreshSecretRotationExcludesOldHolderMaterial(t *testing.T) {
	oldHolder := fixedResolvedCapability(t, 7)
	current := oldHolder
	current.Generation++
	newSecret, ok := identityapi.NewCapabilitySecret(bytes.Repeat([]byte{0x63}, 32))
	require.True(t, ok)
	current.Secret = newSecret

	oldMaterial, err := Derive(oldHolder)
	require.NoError(t, err)
	currentMaterial, err := Derive(current)
	require.NoError(t, err)
	require.NotEqual(t, oldMaterial.ContentTopic, currentMaterial.ContentTopic)
	require.NotEqual(t, oldMaterial.EnvelopeKey(), currentMaterial.EnvelopeKey())
}

func TestDeriveSelectorRejectsEndpointOnlyMaterial(t *testing.T) {
	_, err := Derive(identityapi.ResolvedCapability{Generation: 1})
	require.ErrorContains(t, err, "invalid")
}

func TestPrivacyMaterialFormattingIsRedacted(t *testing.T) {
	material, err := Derive(fixedResolvedCapability(t, 1))
	require.NoError(t, err)
	require.Equal(t, "privacy-material[redacted]", material.String())
	require.NotContains(t, material.String(), material.ContentTopic)
	raw, err := json.Marshal(material)
	require.NoError(t, err)
	require.NotContains(t, string(raw), material.ContentTopic)
	require.NotContains(t, string(raw), stringHex(material.EnvelopeKey()))
}

func fixedResolvedCapability(t *testing.T, generation uint32) identityapi.ResolvedCapability {
	t.Helper()
	secret, ok := identityapi.NewCapabilitySecret(bytes.Repeat([]byte{0x24}, 32))
	require.True(t, ok)
	var channelID [16]byte
	for index := range channelID {
		channelID[index] = 0x11
	}
	return identityapi.ResolvedCapability{
		ChannelID: channelID, Generation: generation, Secret: secret,
	}
}

func stringHex(raw []byte) string {
	const alphabet = "0123456789abcdef"
	out := make([]byte, len(raw)*2)
	for index, value := range raw {
		out[index*2] = alphabet[value>>4]
		out[index*2+1] = alphabet[value&0x0f]
	}
	return string(out)
}
