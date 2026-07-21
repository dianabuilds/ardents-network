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
		API:        APIConfig{ListenAddress: "127.0.0.1:8080", OperatorSubject: "ardentsd-local-api"},
		Network: NetworkConfig{
			TransportProfile: "tcp_only", BindAddress: "0.0.0.0",
			StorePath: "var/ardents/waku-store.db", ReachabilityMode: "private_lan",
			DiscoveryRefreshSeconds: 30,
			Limits: NetworkLimits{
				MaxMessageBytes: 143360, MaxPeerConnections: 64, MaxConnectionsPerIP: 4,
				MaxConcurrentOperations: 16, OperationRate: 20, OperationBurst: 40,
				MaxFilterSubscribers: 32, MaxStoreResults: 128,
			},
		},
		Workloads:     WorkloadsConfig{Executor: "disabled"},
		Data:          DataConfig{MaxReplicaBytes: 1 << 30, DesiredReplicas: 3, MinimumReplicas: 2},
		Logging:       LoggingConfig{Level: "info", Format: "json"},
		Observability: ObservabilityConfig{ListenAddress: "127.0.0.1:9090"},
		Diagnostics:   DiagnosticsConfig{MaxEvents: 1000, DetailLevel: "standard"},
	}
}

const (
	APITokenEnv     = "ARDENTS_API_TOKEN"
	APITokenFileEnv = "ARDENTS_API_TOKEN_FILE"
)

func ResolveDocumentSecrets(doc Document) (Document, error) {
	token := strings.TrimSpace(os.Getenv(APITokenEnv))
	path := strings.TrimSpace(os.Getenv(APITokenFileEnv))
	if token != "" && path != "" {
		return Document{}, fmt.Errorf("configure only one of %s and %s", APITokenEnv, APITokenFileEnv)
	}
	if token != "" {
		doc.API.TokenFile = "environment-secret"
	} else if path != "" {
		doc.API.TokenFile = path
	}
	return doc, nil
}

func APIToken(documentPath string) (string, error) {
	environmentToken := strings.TrimSpace(os.Getenv(APITokenEnv))
	environmentPath := strings.TrimSpace(os.Getenv(APITokenFileEnv))
	if environmentToken != "" && environmentPath != "" {
		return "", fmt.Errorf("configure only one of %s and %s", APITokenEnv, APITokenFileEnv)
	}
	if environmentToken != "" {
		return environmentToken, nil
	}
	path := environmentPath
	if path == "" {
		path = strings.TrimSpace(documentPath)
	}
	if path == "" {
		return "", fmt.Errorf("api.token_file or %s is required", APITokenEnv)
	}
	token, err := readSecretFile(path)
	if err != nil {
		return "", fmt.Errorf("api credential source is unavailable or invalid")
	}
	return token, nil
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
	redactMapValue(out, "api", "token_file")
	redactMapValue(out, "observability", "token_file")
	redactMapValue(out, "network", "private_key_path")
	redactNestedMapValue(out, "network", "wss", "private_key_file")
	for _, field := range []string{"capability_store", "capability_store_key_file", "replay_key_file", "subject"} {
		redactMapValue(out, "privacy", field)
	}
	redactMapEntries(out, "privacy", "trusted_issuers")
	redactNestedMapValue(out, "privacy", "discovery", "reference")
	redactNestedMapValue(out, "privacy", "discovery", "replay_path")
	redactNestedMapValue(out, "privacy", "data", "reference")
	redactNestedMapValue(out, "privacy", "data", "replay_path")
	return out
}

func redactMapEntries(root map[string]any, section, field string) {
	value, ok := root[section].(map[string]any)
	if !ok {
		return
	}
	entries, ok := value[field].(map[string]any)
	if !ok {
		return
	}
	for key := range entries {
		entries[key] = "configured"
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
