package api

import (
	networkparticipation "ardents/internal/network/participation"
	networkreadiness "ardents/internal/network/readiness"
)

func NormalizeNodeProfile(profile NodeProfile) NodeProfile {
	return networkreadiness.NormalizeNodeProfile(profile)
}

func ResolveNodeProfile(profile NodeProfile) (NodeProfileDefinition, error) {
	return networkreadiness.ResolveNodeProfile(profile)
}

func ValidateNodeProfileTransport(nodeProfile NodeProfile, transportProfile Profile) error {
	return networkreadiness.ValidateNodeProfileTransport(nodeProfile, transportProfile)
}

func ResolveBindAddress(explicit string) string {
	return networkparticipation.BindAddress(explicit, BindAddressEnv)
}
