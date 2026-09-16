//go:build linux

package main

import (
	"fmt"
	"path"
	"time"
)

type provisioningDocument struct {
	Schema, RemoteRoot, HostingRoot, NetworkID, AuthorityPublic, AuthorityKey string
	At, NotAfter, Epoch, Inputs, ClosedProfileTemplate                        string
	ReaderPlanTemplate, PublisherPlan, Net32Plan, NodeInventory               string
	IssuerInitialization                                                      string
	IssuerNode                                                                fixtureNode
	State                                                                     []provisioningState
	Services                                                                  []provisioningService
	EntryRoots                                                                []provisioningEntryRoot
}
type provisioningState struct {
	Owner, Host, Root, Materialization string
	MaterializationIndex               uint64
}
type provisioningService struct {
	Owner, Host, Root, Plan, Request string
}
type provisioningEntryRoot struct {
	Owner, Host, Source, Destination string
}

func writeProvisioningInventory(config fixtureConfig, result *generatedPlanIndex, networkID, authority string,
	nodes []fixtureNode, sources []fixtureSource, participants []fixtureParticipant) error {
	document := provisioningDocument{Schema: "ardents-qualification-provisioning-v1", RemoteRoot: result.RemoteRoot, HostingRoot: result.HostingRoot,
		NetworkID: networkID, AuthorityPublic: authority, AuthorityKey: remoteArtifact(result.RemoteRoot, "private/state-authority.pem"),
		At: config.At.Format("2006-01-02T15:04:05Z"), NotAfter: config.At.Add(6 * time.Hour).Format("2006-01-02T15:04:05Z"),
		Epoch: remoteArtifact(result.RemoteRoot, "state/epoch.bin"), Inputs: remoteArtifact(result.RemoteRoot, "state/inputs"),
		ClosedProfileTemplate: result.ClosedProfilePlan, ReaderPlanTemplate: result.ReaderPlanTemplate,
		PublisherPlan: result.PublisherPlan, Net32Plan: result.Net32Plan, NodeInventory: result.NodeInventory,
		IssuerInitialization: result.IssuerInitialization, IssuerNode: nodes[1]}
	for index, item := range nodes {
		document.State = append(document.State, provisioningState{Owner: fmt.Sprintf("node-%02d", index), Host: item.Host,
			Root:                 path.Join(result.RemoteRoot, "state", fmt.Sprintf("node-%02d", index)),
			Materialization:      remoteArtifact(result.RemoteRoot, fmt.Sprintf("state/materializations/%04d.bin", item.MaterializationIndex)),
			MaterializationIndex: item.MaterializationIndex})
	}
	for _, item := range participants {
		document.State = append(document.State, provisioningState{Owner: item.Name, Host: item.Host,
			Root:                 path.Join(result.RemoteRoot, "state", item.Name),
			Materialization:      remoteArtifact(result.RemoteRoot, fmt.Sprintf("state/materializations/%04d.bin", item.MaterializationIndex)),
			MaterializationIndex: item.MaterializationIndex})
		document.Services = append(document.Services, provisioningService{Owner: item.Name, Host: item.Host,
			Root:    path.Join(result.RemoteRoot, "service", item.Name),
			Plan:    path.Join(result.RemoteRoot, "bundle", result.ServiceInitializationPlans[len(document.Services)]),
			Request: path.Join(result.RemoteRoot, "handover", item.Name+"-service.request")})
		document.EntryRoots = append(document.EntryRoots, provisioningEntryRoot{Owner: item.Name, Host: item.Host,
			Source: remoteArtifact(result.RemoteRoot, item.EntryRoot), Destination: remoteArtifact(result.RemoteRoot, item.EntryRoot)})
	}
	for _, item := range sources {
		document.State = append(document.State, provisioningState{Owner: item.Name, Host: item.Host,
			Root:                 path.Join(result.RemoteRoot, "state", item.Name),
			Materialization:      remoteArtifact(result.RemoteRoot, fmt.Sprintf("state/materializations/%04d.bin", item.MaterializationIndex)),
			MaterializationIndex: item.MaterializationIndex})
	}
	if len(document.State) != 23 || len(document.Services) != 5 || len(document.EntryRoots) != 5 {
		return fmt.Errorf("provisioning owner inventory is incomplete")
	}
	result.PreparationInventory = "provisioning-inventory.json"
	return writeJSON(config.Output, result.PreparationInventory, document)
}
