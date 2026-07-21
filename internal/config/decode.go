// Package config owns operator document decoding, defaults, validation, precedence, and change classification.
// It does not own runtime composition or adapter-specific startup.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const MaxDocumentBytes = 1 << 20

func Decode(r io.Reader) (Document, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxDocumentBytes+1))
	if err != nil {
		return Document{}, fmt.Errorf("read operator configuration: %w", err)
	}
	if len(raw) > MaxDocumentBytes {
		return Document{}, fmt.Errorf("operator configuration exceeds %d byte limit", MaxDocumentBytes)
	}
	if err := rejectDuplicateFields(raw); err != nil {
		return Document{}, err
	}
	if err := rejectDeprecatedFields(raw); err != nil {
		return Document{}, err
	}
	doc := Defaults()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("decode operator configuration: %w", err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return Document{}, err
	}
	applyContextDefaults(raw, &doc)
	if err := Validate(doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func requireJSONEnd(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode operator configuration: %w", err)
	}
	return fmt.Errorf("decode operator configuration: multiple JSON values")
}

func rejectDeprecatedFields(raw []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil
	}
	if _, found := root["version"]; found {
		return fmt.Errorf("deprecated field version: use api_version")
	}
	var network map[string]json.RawMessage
	if value, found := root["network"]; found && json.Unmarshal(value, &network) == nil {
		if _, deprecated := network["transport_mode"]; deprecated {
			return fmt.Errorf("deprecated field network.transport_mode: use network.transport_profile")
		}
	}
	return nil
}

func rejectDuplicateFields(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return scanJSONValue(decoder, "$")
}

func scanJSONValue(decoder *json.Decoder, path string) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delim {
	case '{':
		return scanJSONObject(decoder, path)
	case '[':
		return scanJSONArray(decoder, path)
	default:
		return fmt.Errorf("invalid JSON delimiter at %s", path)
	}
}

func scanJSONObject(decoder *json.Decoder, path string) error {
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		name, ok := token.(string)
		if !ok {
			return fmt.Errorf("invalid JSON object key at %s", path)
		}
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("duplicate field %s.%s", path, name)
		}
		seen[name] = struct{}{}
		if err := scanJSONValue(decoder, path+"."+name); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}

func scanJSONArray(decoder *json.Decoder, path string) error {
	for index := 0; decoder.More(); index++ {
		if err := scanJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index)); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envList(name string) []string {
	fields := strings.FieldsFunc(strings.TrimSpace(os.Getenv(name)), func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	items := make([]string, 0, len(fields))
	for _, field := range fields {
		if value := strings.TrimSpace(field); value != "" {
			items = append(items, value)
		}
	}
	return items
}

func envPort(name string) (int, bool, error) {
	value, set, err := envInteger(name)
	if err != nil {
		return 0, false, err
	}
	if set && (value < 0 || value > 65535) {
		return 0, false, fmt.Errorf("%s: port must be between 0 and 65535", name)
	}
	return value, set, nil
}

func envNonNegative(name string) (int, bool, error) {
	value, set, err := envInteger(name)
	if err != nil {
		return 0, false, err
	}
	if set && value < 0 {
		return 0, false, fmt.Errorf("%s: value cannot be negative", name)
	}
	return value, set, nil
}

func envInteger(name string) (int, bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, fmt.Errorf("%s: parse int: %w", name, err)
	}
	return value, true, nil
}

const OperatorFileEnv = "ARDENTS_CONFIG_FILE"

func OperatorFile() string { return strings.TrimSpace(os.Getenv(OperatorFileEnv)) }

func LegacyEnvironment() (Document, string, error) {
	doc := Defaults()
	doc.Node.Name = envOr("ARDENTS_NODE_NAME", "ardd")
	doc.Node.DataDir = envOr("ARDENTS_DATA_DIR", filepath.Join("var", doc.Node.Name))
	doc.Node.Profile = envOr("ARDENTS_NODE_PROFILE", doc.Node.Profile)
	doc.API.ListenAddress = envOr("ARDENTS_ADDR", doc.API.ListenAddress)
	doc.Network.StorePath = envOr("ARDENTS_WAKU_STORE_PATH", filepath.Join(doc.Node.DataDir, "waku-store.db"))
	doc.Network.BindAddress = envOr("ARDENTS_TRANSPORT_BIND_ADDRESS", doc.Network.BindAddress)
	doc.Network.BootstrapPeers = envList("ARDENTS_BOOTSTRAP_PEERS")
	doc.Network.TrustAnchors = envList("ARDENTS_TRUST_ANCHORS")
	doc.Network.TransportProfile = envOr("ARDENTS_TRANSPORT_PROFILE", doc.Network.TransportProfile)
	doc.Network.DNSDiscoveryURLs = envList("ARDENTS_DNS_DISCOVERY_URLS")
	doc.Network.DNSDiscoveryNameServer = strings.TrimSpace(os.Getenv("ARDENTS_DNS_DISCOVERY_NAMESERVER"))
	doc.Network.AdvertiseAddresses = envList("ARDENTS_ADVERTISE_ADDRESSES")
	doc.Network.ReachabilityMode = legacyReachability(doc.Node.Profile)
	if value := strings.TrimSpace(os.Getenv("ARDENTS_REACHABILITY_MODE")); value != "" {
		doc.Network.ReachabilityMode = value
	}

	ports := []struct {
		name   string
		target *int
	}{
		{"ARDENTS_TRANSPORT_PORT", &doc.Network.ListenPort},
		{"ARDENTS_WSS_PORT", &doc.Network.WSS.Port},
	}
	for _, item := range ports {
		value, set, err := envPort(item.name)
		if err != nil {
			return Document{}, "", err
		}
		if set {
			*item.target = value
		}
	}
	doc.Network.WSS.CertificateFile = strings.TrimSpace(os.Getenv("ARDENTS_WSS_CERT_PATH"))
	doc.Network.WSS.PrivateKeyFile = strings.TrimSpace(os.Getenv("ARDENTS_WSS_KEY_PATH"))
	doc.Network.WSS.CAFile = strings.TrimSpace(os.Getenv("ARDENTS_WSS_CA_PATH"))
	doc.Network.WSS.AdvertiseAddress = strings.TrimSpace(os.Getenv("ARDENTS_WSS_ADVERTISE_ADDRESS"))
	if err := applyLegacyLimits(&doc.Network.Limits); err != nil {
		return Document{}, "", err
	}

	doc.Workloads.Executor = strings.ToLower(envOr("ARDENTS_WORKLOAD_EXECUTOR", doc.Workloads.Executor))
	doc.Workloads.AllowedRegistries = envList("ARDENTS_WORKLOAD_ALLOWED_REGISTRIES")
	doc.Workloads.AllowedPolicyRefs = envList("ARDENTS_WORKLOAD_ALLOWED_POLICY_REFS")
	doc.Workloads.TrustedRuntime = strings.TrimSpace(os.Getenv("ARDENTS_WORKLOAD_TRUSTED_RUNTIME"))
	doc.Workloads.UntrustedRuntime = strings.TrimSpace(os.Getenv("ARDENTS_WORKLOAD_UNTRUSTED_RUNTIME"))
	doc.Workloads.AllowedIngressHosts = envList("ARDENTS_WORKLOAD_INGRESS_HOSTS")
	doc.Workloads.IngressBindAddress = strings.TrimSpace(os.Getenv("ARDENTS_WORKLOAD_INGRESS_BIND_ADDRESS"))
	doc.Workloads.IngressProxyImage = strings.TrimSpace(os.Getenv("ARDENTS_WORKLOAD_INGRESS_PROXY_IMAGE"))
	doc.Policy.AllowedPolicyRefs = append([]string(nil), doc.Workloads.AllowedPolicyRefs...)

	if err := Validate(doc); err != nil {
		return Document{}, "", err
	}
	token, err := APIToken("")
	if err != nil {
		return Document{}, "", err
	}
	return doc, token, nil
}

func legacyReachability(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "local_development":
		return "local_only"
	case "constrained_light_client":
		return "outbound_only"
	default:
		return "private_lan"
	}
}

func applyLegacyLimits(limits *NetworkLimits) error {
	bindings := []struct {
		name   string
		target *int
	}{
		{"ARDENTS_MAX_NETWORK_MESSAGE_BYTES", &limits.MaxMessageBytes},
		{"ARDENTS_MAX_PEER_CONNECTIONS", &limits.MaxPeerConnections},
		{"ARDENTS_MAX_CONNECTIONS_PER_IP", &limits.MaxConnectionsPerIP},
		{"ARDENTS_MAX_NETWORK_CONCURRENCY", &limits.MaxConcurrentOperations},
		{"ARDENTS_NETWORK_OPERATION_RATE", &limits.OperationRate},
		{"ARDENTS_NETWORK_OPERATION_BURST", &limits.OperationBurst},
		{"ARDENTS_MAX_FILTER_SUBSCRIBERS", &limits.MaxFilterSubscribers},
		{"ARDENTS_MAX_STORE_RESULTS", &limits.MaxStoreResults},
	}
	for _, item := range bindings {
		value, set, err := envNonNegative(item.name)
		if err != nil {
			return err
		}
		if set {
			*item.target = value
		}
	}
	return nil
}
