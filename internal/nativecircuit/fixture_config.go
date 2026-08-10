package nativecircuit

import (
	"encoding/hex"
	"os"
	"path/filepath"
)

func fixedNativeRoleConfigs(runID string, hops map[string]roleHop, slot handle, leafSHA256, payloadSeed, fault string) map[string]roleConfig {
	connections := map[string]int{
		"user-entry": 2, "user-interior": 2, "rendezvous": 2, "service-interior": 1, "data-service-entry": 1,
		"introduction-forwarder": 1, "introduction-node": 2, "introduction-interior": 1, "introduction-entry": 1,
	}
	allowed := map[string][]string{
		"user-entry":             {hops["user-interior"].Address},
		"user-interior":          {hops["rendezvous"].Address, hops["introduction-forwarder"].Address},
		"service-interior":       {hops["rendezvous"].Address},
		"data-service-entry":     {hops["service-interior"].Address},
		"introduction-forwarder": {hops["introduction-node"].Address},
		"introduction-interior":  {hops["introduction-node"].Address},
		"introduction-entry":     {hops["introduction-interior"].Address},
	}
	configs := make(map[string]roleConfig, len(nativeApplicationRoles))
	for _, role := range nativeNodeRoles {
		configs[role] = roleConfig{
			SchemaVersion: nativeRoleSchema, RunID: runID, Role: role, ListenAddress: ":37001",
			CertificatePath: "/config/node.pem", PrivateKeyPath: "/config/node.key",
			AllowedNext: allowed[role], ExpectedConnections: connections[role],
		}
	}
	configs["user"] = roleConfig{
		SchemaVersion: nativeRoleSchema, RunID: runID, Role: "user", StartPath: "/control/user-start",
		Profile: candidateProfile, Rendezvous: hops["rendezvous"].Address, SlotHex: hex.EncodeToString(slot[:]),
		IntroductionPath: []roleHop{hops["user-entry"], hops["user-interior"], hops["introduction-forwarder"], hops["introduction-node"]},
		DataPath:         []roleHop{hops["user-entry"], hops["user-interior"], hops["rendezvous"]},
		HPKEPublicPath:   "/config/hpke-public.bin", TargetRootPath: "/config/target-root.pem",
		ExpectedLeafSHA256: leafSHA256, PayloadSeed: payloadSeed, PayloadBytes: 64 * 1024, Fault: fault,
	}
	configs["service"] = roleConfig{
		SchemaVersion: nativeRoleSchema, RunID: runID, Role: "service", StartPath: "/control/nodes-ready",
		Profile: candidateProfile, Rendezvous: hops["rendezvous"].Address, SlotHex: hex.EncodeToString(slot[:]),
		IntroductionPath: []roleHop{hops["introduction-entry"], hops["introduction-interior"], hops["introduction-node"]},
		DataPath:         []roleHop{hops["data-service-entry"], hops["service-interior"], hops["rendezvous"]},
		HPKEPrivatePath:  "/config/hpke-private.bin", EndpointCertificate: "/config/instance-chain.pem", EndpointPrivateKey: "/config/instance.key",
	}
	return configs
}

type fixtureToolConfig struct {
	SchemaVersion     string            `json:"schema_version"`
	RunID             string            `json:"run_id"`
	Role              string            `json:"role"`
	Mode              string            `json:"mode"`
	DelayMilliseconds int               `json:"delay_milliseconds,omitempty"`
	Links             []fixtureToolLink `json:"links,omitempty"`
}

type fixtureToolLink struct {
	Name string `json:"name"`
	Peer string `json:"peer"`
}

func prepareNativeToolConfigs(fixture *nativeFixture, runID string) error {
	for _, role := range nativeApplicationRoles {
		name := "shape-" + role
		delay := 0
		if role == "user" || role == "service" {
			delay = 40
		}
		if err := prepareFixtureTool(fixture, name, fixtureToolConfig{
			SchemaVersion: "carrier-lab-native-tool-role/v1", RunID: runID, Role: name, Mode: "shape", DelayMilliseconds: delay,
		}); err != nil {
			return err
		}
	}
	captures := map[string][]fixtureToolLink{
		"user":       {{Name: "user-user-entry", Peer: "user-entry"}},
		"user-entry": {{Name: "user-entry-user-interior", Peer: "user-interior"}},
		"user-interior": {
			{Name: "user-interior-rendezvous", Peer: "rendezvous"},
			{Name: "user-interior-introduction-forwarder", Peer: "introduction-forwarder"},
		},
		"rendezvous":             {{Name: "rendezvous-service-interior", Peer: "service-interior"}},
		"service-interior":       {{Name: "service-interior-data-service-entry", Peer: "data-service-entry"}},
		"data-service-entry":     {{Name: "data-service-entry-service", Peer: "service"}},
		"introduction-forwarder": {{Name: "introduction-forwarder-introduction-node", Peer: "introduction-node"}},
		"introduction-node":      {{Name: "introduction-node-introduction-interior", Peer: "introduction-interior"}},
		"introduction-interior":  {{Name: "introduction-interior-introduction-entry", Peer: "introduction-entry"}},
		"introduction-entry":     {{Name: "introduction-entry-service", Peer: "service"}},
	}
	for role, links := range captures {
		name := "capture-" + role
		if err := prepareFixtureTool(fixture, name, fixtureToolConfig{
			SchemaVersion: "carrier-lab-native-tool-role/v1", RunID: runID, Role: name, Mode: "capture", Links: links,
		}); err != nil {
			return err
		}
	}
	return nil
}

func prepareFixtureTool(fixture *nativeFixture, name string, config fixtureToolConfig) error {
	configDirectory := filepath.Join(fixture.root, "tool-configs", name)
	evidenceDirectory := filepath.Join(fixture.root, "tool-evidence", name)
	for _, directory := range []string{configDirectory, evidenceDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return err
		}
	}
	if err := os.Chmod(evidenceDirectory, 0o777); err != nil {
		return err
	}
	fixture.toolEvidence[name] = evidenceDirectory
	return writeFixtureJSON(filepath.Join(configDirectory, "tool.json"), config)
}
