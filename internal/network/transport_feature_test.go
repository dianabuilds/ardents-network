package network

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransportFeatureParserIsClosedAndCanonical(t *testing.T) {
	for _, value := range []string{
		"relay",
		"store",
		"filter",
		"lightpush",
		"filter_client",
		"lightpush_client",
		"store_client",
		"filter_service",
		"lightpush_service",
		"peer_connectivity",
		"bootstrap_sync",
		"inbound_reachability",
		"endpoint_publication",
		"transport_surface_expansion",
		"profile_recovery_pending",
		"store_recovery",
		"private_publication",
		"private_discovery",
		"private_data_exchange",
	} {
		feature, err := ParseTransportFeature(value)
		require.NoError(t, err, value)
		require.Equal(t, value, feature.String())
		require.True(t, feature.Valid())
	}

	for _, value := range []string{"", " relay", "Relay", "unknown", "relay ", "waku/relay"} {
		_, err := ParseTransportFeature(value)
		require.Error(t, err, value)
	}
}

func TestValidateTransportFeaturesRejectsMalformedAndDuplicates(t *testing.T) {
	require.NoError(t, ValidateTransportFeatures([]TransportFeature{TransportFeatureRelay, TransportFeatureStore}))
	require.Error(t, ValidateTransportFeatures([]TransportFeature{TransportFeature("unknown")}))
	require.Error(t, ValidateTransportFeatures([]TransportFeature{TransportFeatureRelay, TransportFeatureRelay}))
}
