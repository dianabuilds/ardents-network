package fixture

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/network/epoch/assignment"
	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func (fixture nodeFixture) writePlans(root string) error {
	public := hex.EncodeToString(fixture.authorityPublic)
	pins := make([]string, len(fixture.sourceClients))
	for index := range fixture.sourceClients {
		pins[index] = hex.EncodeToString(fixture.sourceClients[index].sourcePin[:])
	}
	for index := range 2 {
		zone := fmt.Sprintf("s%d", index+1)
		plan := map[string]any{"schema": "ardents-h3-source-server-v1", "state_root": "/run/ardents/state",
			"network_id": hex.EncodeToString(fixture.network[:]), "authority_public": []string{public}, "threshold": 1,
			"at": fixture.now.Format(timeFormat), "listen": fmt.Sprintf("0.0.0.0:%d", 4301+index),
			"server_certificate": "/run/ardents/secrets/source-server-cert.pem", "server_key": "/run/ardents/secrets/source-server-key.pem",
			"client_root": "/run/ardents/secrets/source-client-root.pem", "client_key_digests": pins,
			"materialization_index": 0, "runtime_profile": "h3-s-v1"}
		if err := byteio.WriteJSON(filepath.Join(root, "plans", "source-"+fmt.Sprint(index+1)+".json"), plan, 64<<10); err != nil {
			return err
		}
		_ = zone
	}
	for index := range 2 {
		plan := fixture.nodePlan(index, public)
		if err := byteio.WriteJSON(filepath.Join(root, "plans", fmt.Sprintf("node-%d.json", index+1)), plan, 64<<10); err != nil {
			return err
		}
		if index == 0 {
			delete(plan, "node_resource_profile")
			if err := byteio.WriteJSON(filepath.Join(root, "plans", "node-1-emfile.json"), plan, 64<<10); err != nil {
				return err
			}
		}
	}
	if err := byteio.WriteJSON(filepath.Join(root, "plans", "endpoint.json"), fixture.endpointPlan(public), 64<<10); err != nil {
		return err
	}
	nodes := make([]map[string]any, 2)
	for index, record := range fixture.records {
		domain := fixture.selectedDomain(2, fixture.epochs[1].seed, record.family)
		digest := assignment.Digest(fixture.network, 2, fixture.epochs[1].seed, record.family, domain)
		nodes[index] = map[string]any{"address": fmt.Sprintf("172.30.3.%d:%d", 11+index, 4401+index),
			"node_id": hex.EncodeToString(record.nodeID[:]), "assignment_digest": hex.EncodeToString(digest[:]),
			"server_key_digest": hex.EncodeToString(fixture.roleServers[index].pin[:])}
	}
	plan := map[string]any{"schema": "ardents-h3-node-probe-plan-v1", "network_id": hex.EncodeToString(fixture.network[:]),
		"epoch_digest": hex.EncodeToString(fixture.epochs[1].digest[:]), "nodes": nodes}
	return byteio.WriteJSON(filepath.Join(root, "plans", "harness.json"), plan, 64<<10)
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

func (fixture nodeFixture) sourceMembers() []map[string]any {
	members := make([]map[string]any, 2)
	for index := range members {
		identity := nodeSourceIdentity(index)
		members[index] = map[string]any{"address": fmt.Sprintf("172.30.%d.10:%d", index+1, 4301+index),
			"server_name": "source.node", "identity": hex.EncodeToString(identity[:]),
			"family": fmt.Sprintf("source-%d-family", index+1), "endpoint_handle": fmt.Sprintf("source-%d-handle", index+1),
			"root_ca":         fmt.Sprintf("/run/ardents/secrets/source-%d-root.pem", index+1),
			"leaf_key_digest": hex.EncodeToString(fixture.sourceServers[index].sourcePin[:])}
	}
	return members
}

func (fixture nodeFixture) nodePlan(index int, authority string) map[string]any {
	record := fixture.records[index]
	clientPin := fixture.harness.pin
	return map[string]any{"schema": "ardents-h3-node-plan-v1", "state_root": "/run/ardents/state",
		"network_id": hex.EncodeToString(fixture.network[:]), "authority_public": []string{authority}, "threshold": 1,
		"at": fixture.now.Format(timeFormat), "listen": record.endpoint,
		"server_certificate": "/run/ardents/secrets/role-server-cert.pem", "server_key": "/run/ardents/secrets/role-server-key.pem",
		"client_root": "/run/ardents/secrets/harness-root.pem", "client_key_digests": []string{hex.EncodeToString(clientPin[:])},
		"materialization_index": index, "clock_observation_file": "/run/ardents/clock/observation",
		"order_seed": strings.Repeat("44", 32), "source_client_certificate": "/run/ardents/secrets/source-client-cert.pem",
		"source_client_key": "/run/ardents/secrets/source-client-key.pem", "sources": fixture.sourceMembers(),
		"node_id": hex.EncodeToString(record.nodeID[:]), "identity_key": "/run/ardents/secrets/identity-key.pem",
		"maximum_duty_ms": 6000, "drain_timeout_ms": 6000, "node_resource_profile": "h3-np1-v1"}
}

func (fixture nodeFixture) endpointPlan(authority string) map[string]any {
	return map[string]any{"schema": "ardents-h3-source-plan-v1", "network_id": hex.EncodeToString(fixture.network[:]),
		"authority_public": []string{authority}, "threshold": 1, "clock_observed_at": fixture.now.Format(timeFormat),
		"clock_observation_file": "/run/ardents/clock/observation", "order_seed": strings.Repeat("44", 32),
		"materialization_index": 0, "client_certificate": "/run/ardents/secrets/source-client-cert.pem",
		"client_key": "/run/ardents/secrets/source-client-key.pem", "sources": fixture.sourceMembers(),
		"runtime_profile": "h3-s-v1", "refresh_interval_ms": 5000}
}
