package nativecircuit

import "encoding/hex"

func fixedC3RoleConfigs(runID string, hops map[string]roleHop, slot handle, leafSHA256, payloadSeed string, payloadBytes int, workload *nativeWorkload) map[string]roleConfig {
	connections := map[string]int{"user-entry": 2, "rendezvous": 2, "data-service-entry": 2, "introduction-node": 2}
	allowed := map[string][]string{
		"user-entry":         {hops["rendezvous"].Address, hops["introduction-node"].Address},
		"data-service-entry": {hops["rendezvous"].Address, hops["introduction-node"].Address},
	}
	configs := make(map[string]roleConfig, len(c3Topology.applicationRoles))
	for _, role := range c3Topology.nodeRoles {
		configs[role] = roleConfig{
			SchemaVersion: nativeRoleSchema, RunID: runID, Role: role, ListenAddress: ":37001",
			CertificatePath: "/config/node.pem", PrivateKeyPath: "/config/node.key",
			AllowedNext: allowed[role], ExpectedConnections: connections[role],
		}
	}
	configs["user"] = roleConfig{
		SchemaVersion: nativeRoleSchema, RunID: runID, Role: "user", StartPath: "/control/user-start",
		Profile: c3Profile, Rendezvous: hops["rendezvous"].Address, SlotHex: hex.EncodeToString(slot[:]),
		IntroductionPath: []roleHop{hops["user-entry"], hops["introduction-node"]},
		DataPath:         []roleHop{hops["user-entry"], hops["rendezvous"]}, HPKEPublicPath: "/config/hpke-public.bin",
		TargetRootPath: "/config/target-root.pem", ExpectedLeafSHA256: leafSHA256,
		PayloadSeed: payloadSeed, PayloadBytes: payloadBytes,
	}
	configs["service"] = roleConfig{
		SchemaVersion: nativeRoleSchema, RunID: runID, Role: "service", StartPath: "/control/nodes-ready",
		Profile: c3Profile, Rendezvous: hops["rendezvous"].Address, SlotHex: hex.EncodeToString(slot[:]),
		IntroductionPath: []roleHop{hops["data-service-entry"], hops["introduction-node"]},
		DataPath:         []roleHop{hops["data-service-entry"], hops["rendezvous"]}, HPKEPrivatePath: "/config/hpke-private.bin",
		EndpointCertificate: "/config/instance-chain.pem", EndpointPrivateKey: "/config/instance.key",
	}
	if workload.Kind == "stream" {
		for _, role := range []string{"user", "service"} {
			config := configs[role]
			config.PayloadSeed, config.PayloadBytes = "", 0
			config.StreamDirection, config.StreamSeed, config.StreamDuration = workload.Direction, workload.Seed, workload.DurationSeconds
			configs[role] = config
		}
	}
	return configs
}
