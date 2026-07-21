package network

import "slices"

import "fmt"

type NodeProfile string

const (
	NodeProfileServiceNode       NodeProfile = "service_node"
	NodeProfileConstrainedClient NodeProfile = "constrained_light_client"
	NodeProfileLocalDevelopment  NodeProfile = "local_development"
	NodeProfileRestrictedDefense NodeProfile = "restricted_defense"
)

type NodeProfileDefinition struct {
	Profile           NodeProfile
	Selectable        bool
	Implemented       bool
	AutomaticOnly     bool
	AllowedTransports []Profile
}

func NormalizeNodeProfile(profile NodeProfile) NodeProfile {
	if profile == "" {
		return NodeProfileLocalDevelopment
	}
	return profile
}

func LookupNodeProfile(profile NodeProfile) NodeProfileDefinition {
	switch NormalizeNodeProfile(profile) {
	case NodeProfileServiceNode:
		return NodeProfileDefinition{
			Profile: NodeProfileServiceNode, Selectable: true, Implemented: true,
			AllowedTransports: []Profile{ProfileTCPOnly, ProfileTCPWSS},
		}
	case NodeProfileLocalDevelopment:
		return NodeProfileDefinition{
			Profile: NodeProfileLocalDevelopment, Selectable: true, Implemented: true,
			AllowedTransports: []Profile{ProfileTCPOnly},
		}
	case NodeProfileConstrainedClient:
		return NodeProfileDefinition{
			Profile: NodeProfileConstrainedClient, Selectable: true, Implemented: true,
			AllowedTransports: []Profile{ProfileTCPOnly},
		}
	case NodeProfileRestrictedDefense:
		return NodeProfileDefinition{Profile: NodeProfileRestrictedDefense, Implemented: true, AutomaticOnly: true}
	default:
		return NodeProfileDefinition{}
	}
}

func ResolveNodeProfile(profile NodeProfile) (NodeProfileDefinition, error) {
	definition := LookupNodeProfile(profile)
	if definition.Profile == "" {
		return NodeProfileDefinition{}, fmt.Errorf("node profile %q is unknown", profile)
	}
	if definition.AutomaticOnly {
		return NodeProfileDefinition{}, fmt.Errorf("node profile %q is an automatic runtime mode and is not selectable", definition.Profile)
	}
	if !definition.Implemented || !definition.Selectable {
		return NodeProfileDefinition{}, fmt.Errorf("node profile %q is not implemented", definition.Profile)
	}
	return definition, nil
}

func ValidateNodeProfileTransport(nodeProfile NodeProfile, transportProfile Profile) error {
	definition, err := ResolveNodeProfile(nodeProfile)
	if err != nil {
		return err
	}
	resolvedTransport, err := ResolveProfile(transportProfile)
	if err != nil {
		return err
	}
	if slices.Contains(definition.AllowedTransports, resolvedTransport.Profile) {
		return nil
	}
	return fmt.Errorf("node profile %q does not support transport profile %q", definition.Profile, resolvedTransport.Profile)
}
