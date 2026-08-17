//go:build live

package network_test

import "encoding/hex"

func blockedImportPlan(network blockedBridgeNetwork, _ string) map[string]any {
	return map[string]any{"state_root": "/run/state/bridge", "network_state_root": "/run/state/bridge-network",
		"invite_file": "/run/secure/invite.bin", "network_id": liveHex(network.snapshot.NetworkID),
		"network_authorities": []string{hex.EncodeToString(network.authorityPublic)}, "network_threshold": 1,
		"network_profile": "h3-role-probe-v1", "route_profile": "h3-route-tracer-v1",
		"local_role_state_root": "/run/state/local-roles", "time_confidence_file": "/run/secure/time-confidence"}
}
