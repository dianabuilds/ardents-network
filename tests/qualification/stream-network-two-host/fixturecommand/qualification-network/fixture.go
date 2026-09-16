//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

var qualificationRoles = [16][2]uint8{
	{1, 1}, {2, 6}, {1, 2}, {1, 1}, {1, 2}, {2, 5}, {4, 3}, {4, 1},
	{4, 1}, {4, 2}, {4, 2}, {3, 1}, {3, 1}, {3, 2}, {3, 2}, {2, 4},
}

func buildFixture(config fixtureConfig) error {
	seed, err := decodeSeed(config.Seed)
	if err != nil {
		return err
	}
	networkID := sha256.Sum256(append([]byte("ardents-issue60-network-v1"), seed...))
	authority := derivedKey(seed, "state-authority")
	authorityPublic := authority.Public().(ed25519.PublicKey)
	notBefore, notAfter := config.At.Add(-5*time.Minute), config.At.Add(6*time.Hour)
	if err := writePrivateKey(config.Output, "private/state-authority.pem", authority); err != nil {
		return err
	}
	carrier := "ardents-carrier-" + config.Carrier + "-v2"
	records := make([]networkfixture.Record, 0, 23)
	nodeMembers := make([]routeMember, 0, 16)
	nodes := make([]fixtureNode, 0, 16)
	for index, role := range qualificationRoles {
		key := derivedKey(seed, fmt.Sprintf("route-node-%02d", index))
		public := key.Public().(ed25519.PublicKey)
		nodeID := sha256.Sum256(append([]byte("ardents-issue60-node-v1"), public...))
		family := fmt.Sprintf("issue60-route-node-%02d", index)
		host, address := nodeHost(config, index), nodeHostAddress(config, index)
		endpoint := fmt.Sprintf("%s:%d", address, 50000+index)
		record, err := networkfixture.BuildRecord(networkfixture.RecordSpec{
			NetworkID: networkID, NodeID: nodeID, Generation: uint64(index + 1),
			ValidFrom: notBefore, ValidUntil: notAfter, Family: family, Endpoint: endpoint,
			Carrier: carrier, Capability: 2, Capacity: 64, PrivateKey: key,
		})
		if err != nil {
			return fmt.Errorf("build Route Node %d: %w", index, err)
		}
		keyPath := fmt.Sprintf("private/nodes/%02d-key.pem", index)
		certPath := fmt.Sprintf("private/nodes/%02d-cert.pem", index)
		if err := writePrivateKey(config.Output, keyPath, key); err != nil {
			return err
		}
		if err := writeNodeCertificate(config.Output, certPath, fmt.Sprintf("issue60-node-%02d.test", index),
			int64(index+1), key, notBefore, notAfter); err != nil {
			return err
		}
		digest := sha256.Sum256(record.Raw)
		member := entry.ClosedSetMember{NodeID: nodeID, PublicKey: bytes32(public),
			FamilyID: sha256.Sum256([]byte(family)), RecordDigest: digest, DutyGeneration: uint64(index + 1),
			Domain: role[0], NotAfter: notAfter}
		nodeMembers = append(nodeMembers, routeMember{ClosedSetMember: member, subrole: role[1]})
		nodes = append(nodes, fixtureNode{ID: hex32(nodeID), PublicKey: hex.EncodeToString(public), Family: family,
			Endpoint: endpoint, Host: host, RoleDomain: role[0], Subrole: role[1],
			DutyGeneration: uint64(index + 1), MaterializationIndex: uint64(index),
			Key: keyPath, Certificate: certPath, RecordSHA256: hex32(digest)})
		records = append(records, record)
	}
	participants, sourceIdentities, err := buildParticipantRecords(config, seed, networkID, carrier, notBefore, notAfter, &records)
	if err != nil {
		return err
	}
	epoch, err := networkfixture.BuildEpoch(networkfixture.EpochSpec{
		NetworkID: networkID, Number: 1, ValidFrom: notBefore, ValidUntil: notAfter,
		Inputs: recordBodies(records), Accepted: records,
		AssignmentSeed: sha256.Sum256(append([]byte("ardents-issue60-assignment-v1"), seed...)),
		Profile:        "ardents-route-v3", Version: 3, Domains: []string{"issue60-a", "issue60-b"},
		Authorities: []ed25519.PrivateKey{authority},
	})
	if err != nil {
		return fmt.Errorf("build qualification Epoch: %w", err)
	}
	if err := assignMaterializationIndices(records, nodes, participants, sourceIdentities); err != nil {
		return err
	}
	sources, err := buildSourceCredentials(config, seed, sourceIdentities, notBefore, notAfter)
	if err != nil {
		return err
	}
	if err := writeEpoch(config.Output, epoch); err != nil {
		return err
	}
	readerEntry, readerInterior, err := selectRoute(filepath.Join(config.Output, "entry-templates", "reader-0"), networkID, 1, nodeMembers, config.At)
	if err != nil {
		return fmt.Errorf("select reader Route: %w", err)
	}
	for index := 1; index < 4; index++ {
		if err := copyEntryRoot(filepath.Join(config.Output, "entry-templates", "reader-0"),
			filepath.Join(config.Output, "entry-templates", fmt.Sprintf("reader-%d", index))); err != nil {
			return err
		}
	}
	publisherEntry, publisherInterior, err := selectRoute(filepath.Join(config.Output, "entry-templates", "publisher"), networkID, 3, nodeMembers, config.At)
	if err != nil {
		return fmt.Errorf("select Publisher Route: %w", err)
	}
	manifest, err := makeNetworkManifest(config, nodes, readerEntry.NodeID, readerInterior.NodeID,
		publisherInterior.NodeID, publisherEntry.NodeID, nodes[15].ID)
	if err != nil {
		return err
	}
	if err := writeJSON(config.Output, "network-manifest.json", manifest); err != nil {
		return err
	}
	bundle := fixtureBundle{Schema: "ardents-qualification-network-fixture-v1", NetworkID: hex32(networkID),
		AuthorityPublic: hex.EncodeToString(authorityPublic), AuthorityKey: "private/state-authority.pem",
		Carrier: config.Carrier, Cell: config.Cell, Profile: config.Profile, Seed: config.Seed, At: config.At.Format(time.RFC3339),
		NotAfter: notAfter.Format(time.RFC3339), Epoch: "state/epoch.bin", Inputs: "state/inputs",
		EpochSHA256: hex32(epoch.Digest), Nodes: nodes, Participants: participants, Sources: sources,
		ReaderEntry: hex32(readerEntry.NodeID), ReaderInterior: hex32(readerInterior.NodeID),
		PublisherEntry: hex32(publisherEntry.NodeID), PublisherInterior: hex32(publisherInterior.NodeID),
		DataJoin: nodes[15].ID, NetworkManifest: "network-manifest.json"}
	plans, err := writeRuntimePlans(config, bundle.NetworkID, bundle.AuthorityPublic, nodes, sources, participants, bundle.EpochSHA256, manifest)
	if err != nil {
		return err
	}
	bundle.RemoteRoot, bundle.NodeInventory = plans.RemoteRoot, plans.NodeInventory
	bundle.NodePlans, bundle.SourcePlans = plans.NodePlans, plans.SourcePlans
	bundle.HostingRoot, bundle.ClockObservationFile = plans.HostingRoot, plans.ClockObservationFile
	bundle.ReaderPlanTemplate, bundle.PublisherPlan = plans.ReaderPlanTemplate, plans.PublisherPlan
	bundle.Net32Plan = plans.Net32Plan
	bundle.ClosedProfilePlan, bundle.IssuerInitialization = plans.ClosedProfilePlan, plans.IssuerInitialization
	bundle.PreparationInventory = plans.PreparationInventory
	bundle.ServiceInitializationPlans = plans.ServiceInitializationPlans
	return writeJSON(config.Output, "fixture.json", bundle)
}

func nodeHost(config fixtureConfig, index int) string {
	if index <= 7 || index == 15 {
		return "reader"
	}
	return "publisher"
}
func nodeHostAddress(config fixtureConfig, index int) string {
	if nodeHost(config, index) == "reader" {
		return config.ReaderHost
	}
	return config.PublisherHost
}
func bytes32(public ed25519.PublicKey) (value [32]byte) { copy(value[:], public); return value }

func buildParticipantRecords(config fixtureConfig, seed []byte, networkID [32]byte, carrier string,
	from, until time.Time, records *[]networkfixture.Record) ([]fixtureParticipant, []fixtureParticipant, error) {
	participants := make([]fixtureParticipant, 0, 5)
	sources := make([]fixtureParticipant, 0, 2)
	names := []struct{ name, host string }{
		{"reader-0", "reader"}, {"reader-1", "reader"}, {"reader-2", "reader"}, {"reader-3", "reader"},
		{"publisher", "publisher"}, {"source-reader", "reader"}, {"source-publisher", "publisher"},
	}
	for index, item := range names {
		key := derivedKey(seed, item.name+"-state")
		nodeID := sha256.Sum256(key.Public().(ed25519.PublicKey))
		address := config.PublisherHost
		if item.host == "reader" {
			address = config.ReaderHost
		}
		record, err := networkfixture.BuildRecord(networkfixture.RecordSpec{NetworkID: networkID, NodeID: nodeID,
			Generation: uint64(17 + index), ValidFrom: from, ValidUntil: until,
			Family: "issue60-" + item.name, Endpoint: fmt.Sprintf("%s:%d", address, 50100+index),
			Carrier: carrier, Capability: 2, Capacity: 1, PrivateKey: key})
		if err != nil {
			return nil, nil, err
		}
		materialIndex := uint64(len(*records))
		*records = append(*records, record)
		value := fixtureParticipant{Name: item.name, Host: item.host, ID: hex32(nodeID), MaterializationIndex: materialIndex}
		if index < 5 {
			value.EntryRoot = "entry-templates/" + item.name
			participants = append(participants, value)
		} else {
			sources = append(sources, value)
		}
	}
	return participants, sources, nil
}

func recordBodies(records []networkfixture.Record) [][]byte {
	values := make([][]byte, len(records))
	for index := range records {
		values[index] = records[index].Raw
	}
	return values
}
func writeEpoch(root string, epoch networkfixture.Epoch) error {
	if err := writePrivateKeyBytes(root, "state/epoch.bin", epoch.Raw); err != nil {
		return err
	}
	for index, body := range epoch.Inputs {
		if err := writePrivateKeyBytes(root, fmt.Sprintf("state/inputs/%04d.bin", index), body); err != nil {
			return err
		}
	}
	for index, body := range epoch.Materials {
		if err := writePrivateKeyBytes(root, fmt.Sprintf("state/materializations/%04d.bin", index), body); err != nil {
			return err
		}
	}
	return nil
}

func assignMaterializationIndices(records []networkfixture.Record, nodes []fixtureNode, groups ...[]fixtureParticipant) error {
	ordered := append([]networkfixture.Record(nil), records...)
	sort.Slice(ordered, func(i, j int) bool { return bytes.Compare(ordered[i].NodeID[:], ordered[j].NodeID[:]) < 0 })
	indices := make(map[string]uint64, len(ordered))
	for index, record := range ordered {
		indices[hex32(record.NodeID)] = uint64(index)
	}
	for index := range nodes {
		materialization, found := indices[nodes[index].ID]
		if !found {
			return errors.New("Route Node has no Epoch materialization")
		}
		nodes[index].MaterializationIndex = materialization
	}
	for _, group := range groups {
		for index := range group {
			materialization, found := indices[group[index].ID]
			if !found {
				return errors.New("State owner has no Epoch materialization")
			}
			group[index].MaterializationIndex = materialization
		}
	}
	return nil
}
