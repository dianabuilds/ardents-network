package nativecircuit

import "encoding/hex"

func fixedDirectRoleConfigs(runID string, nonce handle, leafSHA256, payloadSeed string, payloadBytes int, workload *nativeWorkload) map[string]roleConfig {
	configs := map[string]roleConfig{
		"user": {
			SchemaVersion: nativeRoleSchema, RunID: runID, Role: "user", StartPath: "/control/user-start",
			Profile: directProfile, DirectAddress: "service:37001", SlotHex: hex.EncodeToString(nonce[:]),
			TargetRootPath: "/config/target-root.pem", ExpectedLeafSHA256: leafSHA256,
			PayloadSeed: payloadSeed, PayloadBytes: payloadBytes,
		},
		"service": {
			SchemaVersion: nativeRoleSchema, RunID: runID, Role: "service", Profile: directProfile,
			DirectAddress: ":37001", SlotHex: hex.EncodeToString(nonce[:]),
			EndpointCertificate: "/config/instance-chain.pem", EndpointPrivateKey: "/config/instance.key",
		},
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
