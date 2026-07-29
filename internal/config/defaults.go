package config

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
)

const Version = "ardents.config/v1"

func Defaults() Document {
	return Document{
		APIVersion: Version,
		Node:       NodeConfig{Name: "ardents", Profile: "service_node", DataDir: "var/ardents"},
		API:        APIConfig{SocketPath: "/run/ardents/control.sock"},
		Network: NetworkConfig{
			TransportProfile: "tcp_only", BindAddress: "0.0.0.0",
			StorePath: "var/ardents/waku-store.db", ReachabilityMode: "outbound_only",
			DiscoveryRefreshSeconds: 30,
			Limits: NetworkLimits{
				MaxMessageBytes: 143360, MaxPeerConnections: 64, MaxConnectionsPerIP: 4,
				MaxConcurrentOperations: 16, OperationRate: 20, OperationBurst: 40,
				MaxFilterSubscribers: 32, MaxStoreResults: 128,
				StoreMaxMessages: 100000, StoreMaxAgeSeconds: 7 * 24 * 60 * 60,
				StoreMaxBytes: 2 << 30,
			},
		},
		Workloads:     WorkloadsConfig{Executor: "disabled"},
		Data:          DataConfig{MaxReplicaBytes: 1 << 30, DesiredReplicas: 3, MinimumReplicas: 2},
		Logging:       LoggingConfig{Level: "info", Format: "json"},
		Observability: ObservabilityConfig{ListenAddress: "127.0.0.1:9090"},
		Diagnostics:   DiagnosticsConfig{MaxEvents: 1000, DetailLevel: "standard"},
	}
}

func ObservabilityToken(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	token, err := readSecretFile(path)
	if err != nil {
		return "", fmt.Errorf("observability credential source is unavailable or invalid")
	}
	return token, nil
}

func readSecretFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("secret source must be a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("secret source permissions must not allow group or other access")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", fmt.Errorf("secret source is empty")
	}
	return token, nil
}

func redactDocument(doc Document) map[string]any {
	raw, err := json.Marshal(doc)
	if err != nil {
		return map[string]any{"configuration": "unavailable"}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{"configuration": "unavailable"}
	}
	redactMapValue(out, "observability", "token_file")
	redactMapValue(out, "network", "private_key_path")
	redactNestedMapValue(out, "network", "wss", "private_key_file")
	for _, field := range []string{"channel_grant_store", "channel_grant_store_key_file", "replay_key_file", "subject"} {
		redactMapValue(out, "privacy", field)
	}
	for _, field := range []string{"store_path", "store_key_file", "signer_file", "successor_signer_file", "checkpoint_repository_path"} {
		redactMapValue(out, "authority", field)
	}
	redactTrustedPrincipalKeys(out)
	redactNestedMapValue(out, "privacy", "discovery", "reference")
	redactNestedMapValue(out, "privacy", "discovery", "replay_path")
	redactNestedMapValue(out, "privacy", "data", "reference")
	redactNestedMapValue(out, "privacy", "data", "replay_path")
	return out
}

func redactTrustedPrincipalKeys(root map[string]any) {
	trust, ok := root["trust"].(map[string]any)
	if !ok {
		return
	}
	principals, ok := trust["principals"].([]any)
	if !ok {
		return
	}
	for _, raw := range principals {
		entry, ok := raw.(map[string]any)
		if ok {
			entry["public_key"] = configuredState(entry["public_key"])
		}
	}
}

func redactMapValue(root map[string]any, section, field string) {
	value, ok := root[section].(map[string]any)
	if !ok {
		return
	}
	value[field] = configuredState(value[field])
}

func redactNestedMapValue(root map[string]any, section, nested, field string) {
	value, ok := root[section].(map[string]any)
	if !ok {
		return
	}
	child, ok := value[nested].(map[string]any)
	if !ok {
		return
	}
	child[field] = configuredState(child[field])
}

func configuredState(value any) string {
	text, _ := value.(string)
	if text == "" {
		return "missing"
	}
	return "configured"
}
