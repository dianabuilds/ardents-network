//go:build linux

package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"time"
)

type qualificationOwnerDocument struct {
	Mode         string
	Participants []qualificationOwnerParticipant
}
type qualificationOwnerParticipant struct {
	Participant              map[string]any
	Role, Profile, Condition uint8
	Seed                     string
	ReaderIndex              int
	Link, HostingRoot        string
}

type closedProfileTemplate struct {
	NetworkID, StateGeneration, EpochDigest, IssuerNodeID, IssuanceAuthorityKey string
	Epoch                                                                       uint64
	NotBefore, NotAfter                                                         string
	Nodes                                                                       []closedProfileNode
	TokenKeys                                                                   []any
}
type closedProfileNode struct {
	NodeID, RecordDigest string
	RoleDomain, Subrole  uint8
	DutyGeneration       uint64
}

func writeParticipantPlans(config fixtureConfig, result *generatedPlanIndex, networkID, authority, epochDigest string,
	nodes []fixtureNode, participants []fixtureParticipant) error {
	profile := uint8(1)
	if config.Profile == "publisher-to-client" {
		profile = 2
	}
	condition := uint8(1)
	if config.Cell == "net14s" {
		condition = 2
	} else if config.Cell == "net14-recovery" {
		condition = 3
	}
	authorityBytes, err := hex.DecodeString(authority)
	if err != nil || len(authorityBytes) != 32 {
		return fmt.Errorf("decode generated State authority")
	}
	networkBytes, err := hex.DecodeString(networkID)
	if err != nil || len(networkBytes) != 32 {
		return fmt.Errorf("decode generated Network identity")
	}
	authorityID := sha256.Sum256(authorityBytes)
	authorities := map[string]string{hex32(authorityID): authority}
	var reader qualificationOwnerDocument
	reader.Mode = "stream"
	var publisher qualificationOwnerDocument
	publisher.Mode = "stream"
	result.ReaderPlanTemplate, result.PublisherPlan = "plans/reader-plan.template.json", "plans/publisher-plan.json"
	for index, item := range participants {
		participant := participantPlan(config, result, item, networkBytes, authorityBytes, authorities)
		owner := qualificationOwnerParticipant{Participant: participant, Profile: profile, Condition: condition,
			Seed: config.Seed, HostingRoot: result.HostingRoot}
		if item.Host == "reader" {
			owner.Role, owner.ReaderIndex = 1, index
			reader.Participants = append(reader.Participants, owner)
		} else {
			owner.Role = 2
			publisher.Participants = append(publisher.Participants, owner)
		}
		servicePlanFile := fmt.Sprintf("plans/service-%s-initialize.json", item.Name)
		servicePlan := map[string]any{"schema": "ardents-service-instance-initialize-v1",
			"root": path.Join(result.RemoteRoot, "service", item.Name), "network_id": networkID,
			"not_before":   config.At.Add(-5 * time.Minute).Format(time.RFC3339),
			"not_after":    config.At.Add(6 * time.Hour).Format(time.RFC3339),
			"request_file": path.Join(result.RemoteRoot, "handover", item.Name+"-service.request")}
		if err := writeJSON(config.Output, servicePlanFile, servicePlan); err != nil {
			return err
		}
		result.ServiceInitializationPlans = append(result.ServiceInitializationPlans, servicePlanFile)
	}
	if len(reader.Participants) != 4 || len(publisher.Participants) != 1 {
		return fmt.Errorf("generated Endpoint owner inventory is incomplete")
	}
	if err := writeJSON(config.Output, result.ReaderPlanTemplate, reader); err != nil {
		return err
	}
	if err := writeJSON(config.Output, result.PublisherPlan, publisher); err != nil {
		return err
	}
	result.Net32Plan = "plans/net32-idle-reader-plan.json"
	base := reader.Participants[0]
	participant := make(map[string]any, len(base.Participant))
	for key, value := range base.Participant {
		participant[key] = value
	}
	net32Owner := "net32-reader"
	net32Root := path.Join(result.RemoteRoot, "endpoint", net32Owner)
	participant["LocalRoleRoot"] = path.Join(net32Root, "roles")
	participant["TokenRoot"] = path.Join(net32Root, "tokens")
	participant["PublicationRoot"] = path.Join(net32Root, "publications")
	participant["ServiceInstanceRoot"] = path.Join(result.RemoteRoot, "service", net32Owner)
	participant["ApplicationAddress"] = "/run/ardents/issue60-" + config.Seed[:12] + "-net32-application.sock"
	participant["AdministrationAddress"] = "/run/ardents/issue60-" + config.Seed[:12] + "-net32-administration.sock"
	participant["BrokerID"] = derivedArray(config.Seed, net32Owner+"-broker")
	participant["ConnectionPrincipal"] = derivedArray(config.Seed, net32Owner+"-connection")
	participant["AdministrationPrincipal"] = derivedArray(config.Seed, net32Owner+"-administration")
	participant["ReaderPermission"] = permissionFiles(result.RemoteRoot, net32Owner, "reader", splitPermissionMaxima(4096))
	participant["PublisherPermission"] = permissionFiles(result.RemoteRoot, net32Owner, "publisher", splitPermissionMaxima(16384))
	net32 := qualificationOwnerDocument{Mode: "net32-idle", Participants: []qualificationOwnerParticipant{{
		Participant: participant, Role: 1, Profile: 1, Condition: 1, Seed: config.Seed,
		ReaderIndex: base.ReaderIndex, HostingRoot: result.HostingRoot,
	}}}
	if err := writeJSON(config.Output, result.Net32Plan, net32); err != nil {
		return err
	}
	result.Net32ServiceInitialization = "plans/service-" + net32Owner + "-initialize.json"
	if err := writeJSON(config.Output, result.Net32ServiceInitialization, map[string]any{
		"schema": "ardents-service-instance-initialize-v1",
		"root":   path.Join(result.RemoteRoot, "service", net32Owner), "network_id": networkID,
		"not_before":   config.At.Add(-5 * time.Minute).Format(time.RFC3339),
		"not_after":    config.At.Add(6 * time.Hour).Format(time.RFC3339),
		"request_file": path.Join(result.RemoteRoot, "handover", net32Owner+"-service.request"),
	}); err != nil {
		return err
	}
	result.ServiceInitializationPlans = append(result.ServiceInitializationPlans, result.Net32ServiceInitialization)
	result.IssuerInitialization = "plans/issuer-initialize.json"
	issuer := nodes[1]
	if err := writeJSON(config.Output, result.IssuerInitialization, map[string]any{
		"schema": "ardents-closed-issuer-initialize-v1", "root": path.Join(result.RemoteRoot, "duty", issuer.ID, "issuer"),
		"network_id": networkID, "node_id": issuer.ID, "identity_key": remoteArtifact(result.RemoteRoot, issuer.Key),
		"not_before": config.At.Format(time.RFC3339),
		"not_after":  config.At.Add(6 * time.Hour).Format(time.RFC3339),
	}); err != nil {
		return err
	}
	result.ClosedProfilePlan = "plans/closed-profile.template.json"
	template := closedProfileTemplate{NetworkID: networkID, StateGeneration: epochDigest, EpochDigest: epochDigest,
		IssuerNodeID: issuer.ID, IssuanceAuthorityKey: "REPLACE_WITH_ADMISSION_AUTHORITY", Epoch: 1,
		NotBefore: config.At.Format(time.RFC3339), NotAfter: config.At.Add(6 * time.Hour).Format(time.RFC3339)}
	for _, item := range nodes {
		template.Nodes = append(template.Nodes, closedProfileNode{NodeID: item.ID, RecordDigest: item.RecordSHA256,
			RoleDomain: item.RoleDomain, Subrole: item.Subrole, DutyGeneration: item.DutyGeneration})
	}
	sort.Slice(template.Nodes, func(i, j int) bool { return template.Nodes[i].NodeID < template.Nodes[j].NodeID })
	return writeJSON(config.Output, result.ClosedProfilePlan, template)
}

func participantPlan(config fixtureConfig, result *generatedPlanIndex, item fixtureParticipant,
	network, authority []byte, authorities map[string]string) map[string]any {
	name := item.Name
	runRoot := "/run/ardents/issue60-" + config.Seed[:12] + "-" + name
	ownerRoot := path.Join(result.RemoteRoot, "endpoint", name)
	networkArray, authorityCopy := bytes32Array(network), append([]byte(nil), authority...)
	participant := map[string]any{
		"Network": map[string]any{"Root": path.Join(result.RemoteRoot, "state", name), "NetworkID": networkArray,
			"Authorities": authorities, "Threshold": 1, "ClosedProfileAuthority": base64.StdEncoding.EncodeToString(authorityCopy),
			"AcceptedProfile": "ardents-route-v3", "ClockObservationFile": result.ClockObservationFile,
			"LocalRoleStateRoot": path.Join(result.RemoteRoot, "state-roles", name)},
		"EntryRoot":     remoteArtifact(result.RemoteRoot, item.EntryRoot),
		"LocalRoleRoot": path.Join(ownerRoot, "roles"), "TokenRoot": path.Join(ownerRoot, "tokens"),
		"PublicationRoot": path.Join(ownerRoot, "publications"), "ServiceInstanceRoot": path.Join(result.RemoteRoot, "service", name),
		"ApplicationAddress": runRoot + "-application.sock", "AdministrationAddress": runRoot + "-administration.sock",
		"BrokerID":                derivedArray(config.Seed, name+"-broker"),
		"ConnectionPrincipal":     derivedArray(config.Seed, name+"-connection"),
		"AdministrationPrincipal": derivedArray(config.Seed, name+"-administration"),
		"ReaderPermission":        permissionFiles(result.RemoteRoot, name, "reader", splitPermissionMaxima(1024)),
		"PublisherPermission":     permissionFiles(result.RemoteRoot, name, "publisher", splitPermissionMaxima(16384)),
	}
	return participant
}

func splitPermissionMaxima(total uint32) [3]uint32 {
	base := total / 3
	return [3]uint32{base, base, total - 2*base}
}

func permissionFiles(root, name, role string, maxima [3]uint32) map[string]any {
	base := path.Join(root, "handover", name+"-"+role)
	return map[string]any{"RequestPath": base + ".request", "ResponsePath": base + ".response", "Maxima": maxima}
}

func derivedArray(seedHex, label string) [32]byte {
	seed, _ := hex.DecodeString(seedHex)
	input := append([]byte("ardents-issue60-participant-v1"), 0)
	input = append(input, seed...)
	input = append(input, 0)
	input = append(input, label...)
	return sha256.Sum256(input)
}

func bytes32Array(value []byte) (result [32]byte) {
	copy(result[:], value)
	return result
}
