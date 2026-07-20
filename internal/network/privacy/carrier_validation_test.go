package privacy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCarrierValidationAcceptsSealedEnvelope(t *testing.T) {
	fixture := newEnvelopeFixture(t, true)
	sealed := fixture.seal(t, []byte("private"))

	require.NoError(t, ValidateCarrierEnvelope(sealed))
}

func TestCarrierValidationDetectsReadableSelectorMutation(t *testing.T) {
	err := ValidateOpaqueSelector(DefaultPubsubTopic, "/ardents/1/discovery-record/proto")

	require.Equal(t, CodeSelectorMalformed, CodeOf(err))
}

func TestCarrierValidationDetectsPlaintextPayloadMutation(t *testing.T) {
	err := ValidateEncryptedPayload([]byte(`{"principal":"visible"}`))

	require.Equal(t, CodeEnvelopeMalformed, CodeOf(err))
}
