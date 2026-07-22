package migration

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type InventoryManifest struct {
	SchemaVersion uint32              `json:"schema_version"`
	AuthorityDir  string              `json:"authority_dir"`
	Nodes         []InventoryNodeSpec `json:"nodes"`
}
type InventoryNodeSpec struct {
	Name       string `json:"name"`
	DataDir    string `json:"data_dir"`
	SecretDir  string `json:"secret_dir"`
	ConfigPath string `json:"config_path"`
}
type NetworkInventory struct {
	SchemaVersion        uint32                 `json:"schema_version"`
	Realm                RealmInventory         `json:"realm"`
	Nodes                []NetworkNodeInventory `json:"nodes"`
	ExternalCoordination []string               `json:"external_coordination"`
}
type NetworkNodeInventory struct {
	Name         string              `json:"name"`
	Identity     InventoryReport     `json:"identity"`
	Config       ConfigInventory     `json:"config"`
	Capabilities CapabilityInventory `json:"capabilities"`
	Discovery    DiscoveryInventory  `json:"discovery"`
	Product      ProductInventory    `json:"product"`
}

func RunInventory(args []string, stdout io.Writer) error {
	set := flag.NewFlagSet("ardentsd identity-migration inventory", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	manifestPath := set.String("manifest", "", "inventory manifest JSON")
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *manifestPath == "" {
		return fmt.Errorf("--manifest is required")
	}
	raw, err := os.ReadFile(*manifestPath)
	if err != nil {
		return fmt.Errorf("inventory manifest is unavailable")
	}
	var manifest InventoryManifest
	if err := decodeStrict(raw, &manifest); err != nil || manifest.SchemaVersion != 1 || manifest.AuthorityDir == "" || len(manifest.Nodes) == 0 {
		return fmt.Errorf("inventory manifest is invalid or unsupported")
	}
	report, err := InventoryNetwork(manifest)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func InventoryNetwork(manifest InventoryManifest) (NetworkInventory, error) {
	mappings := map[string]string{}
	report := NetworkInventory{SchemaVersion: InventorySchemaVersion, ExternalCoordination: []string{"expire or refetch remote signed discovery records; never forge the remote issuer signature", "expire retained remote private-message envelopes from the old epoch; mixed p_/p1_ senders reject safely", "coordinate every private-network Node and realm authority before apply; no rolling or dual-ID start"}}
	for _, node := range manifest.Nodes {
		if node.Name == "" || node.DataDir == "" || node.SecretDir == "" || node.ConfigPath == "" {
			return NetworkInventory{}, fmt.Errorf("inventory Node specification is incomplete")
		}
		identity, err := InventoryNodeIdentity(node.DataDir)
		if err != nil {
			return NetworkInventory{}, fmt.Errorf("inventory Node %s identity: %w", node.Name, err)
		}
		if current, exists := mappings[identity.Node.LegacyPrincipal]; exists && current != identity.Node.PrincipalV1 {
			return NetworkInventory{}, fmt.Errorf("legacy Principal mapping conflicts across Nodes")
		}
		mappings[identity.Node.LegacyPrincipal] = identity.Node.PrincipalV1
		report.Nodes = append(report.Nodes, NetworkNodeInventory{Name: node.Name, Identity: identity})
	}
	realm, err := InventoryRealm(manifest.AuthorityDir, mappings)
	if err != nil {
		return NetworkInventory{}, err
	}
	report.Realm = realm
	mappings[realm.LegacyIssuer] = realm.IssuerV1
	for index, node := range manifest.Nodes {
		configReport, err := InventoryConfig(node.ConfigPath, mappings)
		if err != nil {
			return NetworkInventory{}, fmt.Errorf("inventory Node %s configuration: %w", node.Name, err)
		}
		if err := inventoryLocalRealmNode(filepath.Join(node.SecretDir, "local-realm-node.json"), report.Nodes[index].Identity.Node.LegacyPrincipal, realm.LegacyIssuer); err != nil {
			return NetworkInventory{}, fmt.Errorf("inventory Node %s realm state: %w", node.Name, err)
		}
		capabilityReport, err := InventoryCapabilityStore(filepath.Join(node.DataDir, "capabilities.db"), filepath.Join(node.SecretDir, "capability-store.key"), mappings, configReport.IssuerKeys, realm.LegacyIssuer)
		if err != nil {
			return NetworkInventory{}, fmt.Errorf("inventory Node %s capabilities: %w", node.Name, err)
		}
		discoveryReport, err := InventoryDiscovery(filepath.Join(node.DataDir, "ardents.db"), mappings)
		if err != nil {
			return NetworkInventory{}, fmt.Errorf("inventory Node %s discovery: %w", node.Name, err)
		}
		productReport, err := InventoryProductState(filepath.Join(node.DataDir, "ardents.db"), mappings)
		if err != nil {
			return NetworkInventory{}, fmt.Errorf("inventory Node %s product state: %w", node.Name, err)
		}
		report.Nodes[index].Config = configReport
		report.Nodes[index].Capabilities = capabilityReport
		report.Nodes[index].Discovery = discoveryReport
		report.Nodes[index].Product = productReport
	}
	return report, nil
}

func inventoryLocalRealmNode(path, subject, issuer string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("local realm Node state is unavailable")
	}
	var state realmNodeState
	if err := decodeStrict(raw, &state); err != nil || state.Version != "ardents.local-realm-node/v1" || state.Subject != subject || state.Issuer != issuer || !validRealmGrant(state.Discovery) || !validRealmGrant(state.Data) {
		return fmt.Errorf("local realm Node state is inconsistent")
	}
	return nil
}
