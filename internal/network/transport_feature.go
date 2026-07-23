package network

import "fmt"

// TransportFeature names one closed v1 transport or private-messaging
// property. It is not an Access Grant, workload requirement, or channel grant.
type TransportFeature string

const (
	TransportFeatureRelay                  TransportFeature = "relay"
	TransportFeatureStore                  TransportFeature = "store"
	TransportFeatureFilter                 TransportFeature = "filter"
	TransportFeatureLightpush              TransportFeature = "lightpush"
	TransportFeatureFilterClient           TransportFeature = "filter_client"
	TransportFeatureLightpushClient        TransportFeature = "lightpush_client"
	TransportFeatureStoreClient            TransportFeature = "store_client"
	TransportFeatureFilterService          TransportFeature = "filter_service"
	TransportFeatureLightpushService       TransportFeature = "lightpush_service"
	TransportFeaturePeerConnectivity       TransportFeature = "peer_connectivity"
	TransportFeatureBootstrapSync          TransportFeature = "bootstrap_sync"
	TransportFeatureInboundReachability    TransportFeature = "inbound_reachability"
	TransportFeatureEndpointPublication    TransportFeature = "endpoint_publication"
	TransportFeatureSurfaceExpansion       TransportFeature = "transport_surface_expansion"
	TransportFeatureProfileRecoveryPending TransportFeature = "profile_recovery_pending"
	TransportFeatureStoreRecovery          TransportFeature = "store_recovery"
	TransportFeaturePrivatePublication     TransportFeature = "private_publication"
	TransportFeaturePrivateDiscovery       TransportFeature = "private_discovery"
	TransportFeaturePrivateDataExchange    TransportFeature = "private_data_exchange"
)

var transportFeatures = map[TransportFeature]struct{}{
	TransportFeatureRelay: {}, TransportFeatureStore: {}, TransportFeatureFilter: {}, TransportFeatureLightpush: {},
	TransportFeatureFilterClient: {}, TransportFeatureLightpushClient: {}, TransportFeatureStoreClient: {},
	TransportFeatureFilterService: {}, TransportFeatureLightpushService: {}, TransportFeaturePeerConnectivity: {},
	TransportFeatureBootstrapSync: {}, TransportFeatureInboundReachability: {}, TransportFeatureEndpointPublication: {},
	TransportFeatureSurfaceExpansion: {}, TransportFeatureProfileRecoveryPending: {}, TransportFeatureStoreRecovery: {},
	TransportFeaturePrivatePublication: {}, TransportFeaturePrivateDiscovery: {}, TransportFeaturePrivateDataExchange: {},
}

func ParseTransportFeature(value string) (TransportFeature, error) {
	feature := TransportFeature(value)
	if !feature.Valid() {
		return "", fmt.Errorf("invalid transport feature")
	}
	return feature, nil
}

func (f TransportFeature) String() string { return string(f) }

func (f TransportFeature) Valid() bool {
	_, ok := transportFeatures[f]
	return ok
}

func ValidateTransportFeatures(values []TransportFeature) error {
	seen := make(map[TransportFeature]struct{}, len(values))
	for _, value := range values {
		if !value.Valid() {
			return fmt.Errorf("invalid transport feature")
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("duplicate transport feature")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func TransportFeatureStrings(values []TransportFeature) ([]string, error) {
	if err := ValidateTransportFeatures(values); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.String())
	}
	return out, nil
}
