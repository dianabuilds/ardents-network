//go:build linux

package main

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

type generatedPlanIndex struct {
	RemoteRoot, NodeInventory, HostingRoot, ClockObservationFile           string
	NodePlans, SourcePlans                                                 []string
	ReaderPlanTemplate, PublisherPlan, Net32Plan, ClosedProfilePlan        string
	IssuerInitialization, PreparationInventory, Net32ServiceInitialization string
	ServiceInitializationPlans                                             []string
}

type sourcePlanDocument struct {
	Schema                string   `json:"schema"`
	StateRoot             string   `json:"state_root"`
	LocalRoleStateRoot    string   `json:"local_role_state_root"`
	NetworkID             string   `json:"network_id"`
	AuthorityPublic       []string `json:"authority_public"`
	Threshold             int      `json:"threshold"`
	At                    string   `json:"at"`
	Listen                string   `json:"listen"`
	ServerCertificate     string   `json:"server_certificate"`
	ServerKey             string   `json:"server_key"`
	ClientRoot            string   `json:"client_root"`
	ClientKeyDigests      []string `json:"client_key_digests"`
	MaterializationIndex  uint64   `json:"materialization_index"`
	StateProfile          string   `json:"state_profile"`
	StateProfileAuthority string   `json:"state_profile_authority"`
}

type sourceBindingDocument struct {
	Address        string `json:"address"`
	ServerName     string `json:"server_name"`
	Identity       string `json:"identity"`
	Family         string `json:"family"`
	EndpointHandle string `json:"endpoint_handle"`
	RootCA         string `json:"root_ca"`
	LeafKeyDigest  string `json:"leaf_key_digest"`
}

type nodeInventoryDocument struct {
	Schema  string
	Nodes   []nodeInventoryItem
	Sources []sourceInventoryItem
}
type nodeInventoryItem struct{ ID, Host, Plan string }
type sourceInventoryItem struct{ ID, Host, Endpoint, Plan string }

func writeRuntimePlans(config fixtureConfig, networkID, authority string, nodes []fixtureNode,
	sources []fixtureSource, participants []fixtureParticipant, epochDigest string, manifest networkManifest) (generatedPlanIndex, error) {
	if len(nodes) != 16 || len(sources) != 2 || len(manifest.Relays) != 6 {
		return generatedPlanIndex{}, errors.New("runtime plan input is incomplete")
	}
	remoteRoot := "/var/lib/ardents/qualification/issue60-" + config.Seed[:12] + "-" + config.Carrier + "-" + config.Cell + "-" + config.Profile
	hostingRoot := path.Join("/var/lib/ardents/qualification", "issue60-"+config.Seed[:12]+"-hosting")
	clockFile := path.Join(remoteRoot, "clock", "observation")
	result := generatedPlanIndex{RemoteRoot: remoteRoot, NodeInventory: "node-inventory.json",
		HostingRoot: hostingRoot, ClockObservationFile: clockFile}
	inventory := nodeInventoryDocument{Schema: "ardents-qualification-node-inventory-v2"}

	for _, item := range sources {
		planFile := fmt.Sprintf("plans/source-%s.json", strings.TrimPrefix(item.Name, "source-"))
		plan := sourcePlanDocument{Schema: "ardents-source-server-v1",
			StateRoot: path.Join(remoteRoot, "state", item.Name), LocalRoleStateRoot: path.Join(remoteRoot, "roles", item.Name),
			NetworkID: networkID, AuthorityPublic: []string{authority}, Threshold: 1, At: config.At.Format("2006-01-02T15:04:05Z"),
			Listen: item.Endpoint, ServerCertificate: remoteArtifact(remoteRoot, item.ServerCertificate),
			ServerKey: remoteArtifact(remoteRoot, item.ServerKey), ClientRoot: remoteArtifact(remoteRoot, item.ClientRootCA),
			ClientKeyDigests: []string{item.ClientKeyDigest}, MaterializationIndex: item.MaterializationIndex,
			StateProfile: "ardents-route-v3", StateProfileAuthority: authority}
		if err := writeJSON(config.Output, planFile, plan); err != nil {
			return generatedPlanIndex{}, err
		}
		result.SourcePlans = append(result.SourcePlans, planFile)
		inventory.Sources = append(inventory.Sources, sourceInventoryItem{ID: item.ID, Host: item.Host, Endpoint: item.Endpoint, Plan: planFile})
	}

	bindings := make([]sourceBindingDocument, 0, 2)
	for _, item := range sources {
		bindings = append(bindings, sourceBindingDocument{Address: item.Endpoint, ServerName: item.ServerName,
			Identity: item.ID, Family: item.Name, EndpointHandle: "issue60-" + item.Name,
			RootCA: remoteArtifact(remoteRoot, item.ServerRootCA), LeafKeyDigest: item.ServerLeafKeyDigest})
	}
	for index, item := range nodes {
		planFile := fmt.Sprintf("plans/node-%02d.json", index)
		plan := map[string]any{
			"schema": "ardents-node-plan-v1", "state_root": path.Join(remoteRoot, "state", fmt.Sprintf("node-%02d", index)),
			"local_role_state_root": path.Join(remoteRoot, "roles", fmt.Sprintf("node-%02d", index)),
			"network_id":            networkID, "authority_public": []string{authority}, "threshold": 1,
			"clock_observation_file": clockFile, "order_seed": config.Seed,
			"source_client_certificate": remoteArtifact(remoteRoot, sources[0].ClientCertificate),
			"source_client_key":         remoteArtifact(remoteRoot, sources[0].ClientKey), "sources": bindings,
			"node_id": item.ID, "identity_key": remoteArtifact(remoteRoot, item.Key),
			"server_certificate": remoteArtifact(remoteRoot, item.Certificate), "server_key": remoteArtifact(remoteRoot, item.Key),
			"materialization_index": item.MaterializationIndex, "hosting_root": hostingRoot,
			"closed_profile_authority": authority, "drain_timeout_ms": 5000,
		}
		if target := privateListenTarget(manifest, item.Endpoint); target != "" {
			plan["closed_listen_private_override"] = target
		}
		addClosedDuty(plan, item, remoteRoot, hostingRoot, manifest)
		if err := writeJSON(config.Output, planFile, plan); err != nil {
			return generatedPlanIndex{}, err
		}
		result.NodePlans = append(result.NodePlans, planFile)
		inventory.Nodes = append(inventory.Nodes, nodeInventoryItem{ID: item.ID, Host: item.Host, Plan: planFile})
	}
	if err := writeJSON(config.Output, result.NodeInventory, inventory); err != nil {
		return generatedPlanIndex{}, err
	}
	if err := writeParticipantPlans(config, &result, networkID, authority, epochDigest, nodes, participants); err != nil {
		return generatedPlanIndex{}, err
	}
	if err := writeProvisioningInventory(config, &result, networkID, authority, nodes, sources, participants); err != nil {
		return generatedPlanIndex{}, err
	}
	return result, nil
}

func addClosedDuty(plan map[string]any, item fixtureNode, remoteRoot, hostingRoot string, manifest networkManifest) {
	base := path.Join(remoteRoot, "duty", item.ID)
	limits := map[string]any{"connection_limit": 16, "drain_timeout_ms": 5000}
	switch {
	case item.RoleDomain == 2 && item.Subrole == 6:
		limits["root"], limits["admission_root"] = path.Join(base, "issuer"), path.Join(base, "admission")
		plan["closed_issuer"] = limits
	case item.RoleDomain == 2 && item.Subrole == 5:
		limits["root"], limits["admission_root"] = path.Join(base, "descriptors"), path.Join(base, "admission")
		plan["closed_resolution"] = limits
	case item.RoleDomain == 4 && item.Subrole == 3:
		limits["admission_root"] = path.Join(base, "admission")
		plan["closed_introduction"] = limits
	case item.RoleDomain == 2 && item.Subrole == 4:
		limits["hosting_root"], limits["admission_root"] = hostingRoot, path.Join(base, "admission")
		plan["closed_data_join"] = limits
	default:
		limits["root"], limits["hosting_root"] = path.Join(base, "spends"), hostingRoot
		limits["admission_traffic"] = map[string]uint64{"tx": 64 << 20, "rx": 64 << 20}
		limits["termination_traffic"] = map[string]uint64{"tx": 1 << 20, "rx": 1 << 20}
		if item.ID == manifest.Paths[0].Segments[4].From {
			limits["carrier_relay_endpoint"] = manifest.Relays[3].PublicEndpoint
		}
		plan["closed_forwarding"] = limits
	}
}

func privateListenTarget(manifest networkManifest, publicEndpoint string) string {
	for _, relay := range manifest.Relays {
		if relay.PublicEndpoint == publicEndpoint {
			return relay.Target
		}
	}
	return ""
}

func remoteArtifact(remoteRoot, relative string) string {
	return path.Join(remoteRoot, "bundle", relative)
}
