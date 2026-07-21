package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObservabilityListenerMustRemainLoopback(t *testing.T) {
	doc := Defaults()
	doc.Observability.ListenAddress = "0.0.0.0:9090"
	require.ErrorContains(t, Validate(doc), "observability.listen_address must be loopback")

	doc.Observability.TokenFile = "scrape-token"
	require.ErrorContains(t, Validate(doc), "observability.listen_address must be loopback")
}

func TestEffectiveObservabilityTokenReferenceIsRedacted(t *testing.T) {
	doc := Defaults()
	doc.Observability.TokenFile = "private/scrape-token"
	raw, err := json.Marshal(redactDocument(doc))
	require.NoError(t, err)
	require.Contains(t, string(raw), `"observability":{"listen_address":"127.0.0.1:9090","token_file":"configured"}`)
	require.NotContains(t, string(raw), "private/scrape-token")
}

func TestEffectiveApplicationCredentialReferenceIsRedacted(t *testing.T) {
	doc := Defaults()
	doc.ApplicationInterface.TokenFile = "private/application-token"
	raw, err := json.Marshal(redactDocument(doc))
	require.NoError(t, err)
	require.Contains(t, string(raw), `"application_interface":{"enabled":false,"token_file":"configured"}`)
	require.NotContains(t, string(raw), "private/application-token")
}
