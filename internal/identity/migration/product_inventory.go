package migration

import (
	"fmt"
	"sort"
	"strings"

	contentcatalog "ardents/internal/content/catalog"
	"ardents/internal/replication/availability"
	"ardents/internal/replication/placement"
)

type ProductInventory struct {
	Occurrences []Occurrence `json:"occurrences"`
}
type contentSnapshot struct {
	Objects   map[string]contentcatalog.Object             `json:"objects"`
	Blobs     map[string]contentcatalog.Blob               `json:"blobs"`
	Sources   map[string][]contentcatalog.BlobSourceRecord `json:"sources"`
	Manifests map[string]contentcatalog.Manifest           `json:"manifests"`
}
type replicationSnapshot struct {
	Placement    placement.State    `json:"placement"`
	Availability availability.State `json:"availability"`
}

func InventoryProductState(storePath string, mappings map[string]string) (ProductInventory, error) {
	report := ProductInventory{}
	raw, err := readBoltValue(storePath, "data", "snapshot")
	if err != nil {
		return ProductInventory{}, fmt.Errorf("content state is unavailable")
	}
	var content contentSnapshot
	if err := decodeStrict(raw, &content); err != nil {
		return ProductInventory{}, fmt.Errorf("content state has an unknown or invalid schema")
	}
	objectKeys := sortedKeys(content.Objects)
	for _, key := range objectKeys {
		item := content.Objects[key]
		occ, err := ownerOccurrence("ardents.db/data/snapshot/objects/"+key+"/owner", item.Owner, mappings)
		if err != nil {
			return ProductInventory{}, err
		}
		report.Occurrences = append(report.Occurrences, occ)
	}
	manifestKeys := sortedKeys(content.Manifests)
	for _, key := range manifestKeys {
		item := content.Manifests[key]
		occ, err := ownerOccurrence("ardents.db/data/snapshot/manifests/"+key+"/owner", item.Owner, mappings)
		if err != nil {
			return ProductInventory{}, err
		}
		report.Occurrences = append(report.Occurrences, occ)
	}
	sourceKeys := sortedKeys(content.Sources)
	for _, blob := range sourceKeys {
		for index, source := range content.Sources[blob] {
			if source.NodeID == "" {
				continue
			}
			mapped, ok := mappings[source.NodeID]
			if !ok {
				return ProductInventory{}, fmt.Errorf("content Blob source Node has no verified Principal mapping")
			}
			report.Occurrences = append(report.Occurrences, Occurrence{Location: fmt.Sprintf("ardents.db/data/snapshot/sources/%s/%d/node_id", blob, index), Classification: "rebuildable_discovery_projection", LegacyID: source.NodeID, PrincipalV1: mapped})
		}
	}
	repRaw, err := readBoltValue(storePath, "replication", "state")
	if err != nil {
		return ProductInventory{}, fmt.Errorf("replication state is unavailable")
	}
	var replication replicationSnapshot
	if err := decodeStrict(repRaw, &replication); err != nil {
		return ProductInventory{}, fmt.Errorf("replication state has an unknown or invalid schema")
	}
	reservationKeys := sortedKeys(replication.Placement.Reservations)
	for _, key := range reservationKeys {
		peer := replication.Placement.Reservations[key].PeerID
		occ, err := nodeTargetOccurrence("ardents.db/replication/state/reservations/"+key+"/peer_id", peer, mappings)
		if err != nil {
			return ProductInventory{}, err
		}
		report.Occurrences = append(report.Occurrences, occ)
	}
	commitmentKeys := sortedKeys(replication.Placement.Commitments)
	for _, key := range commitmentKeys {
		peer := replication.Placement.Commitments[key].PeerID
		occ, err := nodeTargetOccurrence("ardents.db/replication/state/commitments/"+key+"/peer_id", peer, mappings)
		if err != nil {
			return ProductInventory{}, err
		}
		report.Occurrences = append(report.Occurrences, occ)
	}
	sort.Slice(report.Occurrences, func(i, j int) bool { return report.Occurrences[i].Location < report.Occurrences[j].Location })
	return report, nil
}
func ownerOccurrence(location, owner string, m map[string]string) (Occurrence, error) {
	if !strings.HasPrefix(owner, "p_") {
		return Occurrence{}, fmt.Errorf("content owner is ambiguous")
	}
	mapped, ok := m[owner]
	if !ok {
		return Occurrence{}, fmt.Errorf("content owner has no verified Principal mapping")
	}
	return Occurrence{Location: location, Classification: "rewrite_owner_metadata_and_rebuild_derived_manifest", LegacyID: owner, PrincipalV1: mapped}, nil
}
func nodeTargetOccurrence(location, node string, m map[string]string) (Occurrence, error) {
	mapped, ok := m[node]
	if node == "" || !ok {
		return Occurrence{}, fmt.Errorf("replication target has no verified Principal mapping")
	}
	return Occurrence{Location: location, Classification: "rewrite_node_principal_metadata", LegacyID: node, PrincipalV1: mapped}, nil
}
