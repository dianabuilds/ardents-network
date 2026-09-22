package state_test

import (
	"crypto/ed25519"
	"encoding/hex"
	"time"
)

func sourceServerPlanFixture(network [32]byte, authority ed25519.PublicKey, now time.Time, stateRoot, roleRoot, address string,
	server processCert, clientRoot string, clientPin [32]byte,
) map[string]any {
	return map[string]any{"schema": "ardents-source-server-v1", "state_root": stateRoot, "local_role_state_root": roleRoot,
		"network_id": hex.EncodeToString(network[:]), "authority_public": []string{hex.EncodeToString(authority)}, "threshold": 1,
		"at": now.Format(time.RFC3339), "listen": address, "server_certificate": server.certificate, "server_key": server.key,
		"client_root": clientRoot, "client_key_digests": []string{hex.EncodeToString(clientPin[:])}, "materialization_index": 0}
}
