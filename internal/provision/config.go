package provision

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"

	runtimeconfig "ardents/internal/config"
	"ardents/internal/storage"
)

func operatorDocument(configured options, provisioned NodeProvision) runtimeconfig.Document {
	runtimeDataDir := configured.runtimeDataDir
	if runtimeDataDir == "" {
		runtimeDataDir = "/var/lib/ardents"
	}
	runtimeSecretDir := configured.runtimeSecretDir
	if runtimeSecretDir == "" {
		runtimeSecretDir = "/run/ardents"
	}
	doc := runtimeconfig.Defaults()
	doc.Node.Name = configured.nodeName
	doc.Node.DataDir = runtimeDataDir
	doc.API.TokenFile = filepath.Join(runtimeSecretDir, "api-token")
	doc.API.SocketPath = filepath.Join(runtimeSecretDir, "control.sock")
	doc.API.OperatorSubject = "local-deployment-operator"
	doc.Network.ListenPort = configured.transportPort
	doc.Network.StorePath = filepath.Join(runtimeDataDir, "waku-store.db")
	if configured.bootstrapPeer != "" {
		doc.Network.BootstrapPeers = []string{configured.bootstrapPeer}
	}
	doc.Privacy = runtimeconfig.PrivacyConfig{
		Required: true, CapabilityStore: filepath.Join(runtimeDataDir, "capabilities.db"),
		CapabilityStoreKeyFile: filepath.Join(runtimeSecretDir, "capability-store.key"),
		ReplayKeyFile:          filepath.Join(runtimeSecretDir, "replay.key"), Subject: provisioned.Subject,
		TrustedIssuers: map[string]string{
			provisioned.Issuer: base64.StdEncoding.EncodeToString(provisioned.IssuerPublic),
		},
		Discovery: runtimeconfig.PrivacyChannelConfig{
			Reference: string(provisioned.DiscoveryRef), ReplayPath: filepath.Join(runtimeDataDir, "discovery-replay.db"),
		},
		Data: runtimeconfig.PrivacyChannelConfig{
			Reference: string(provisioned.DataRef), ReplayPath: filepath.Join(runtimeDataDir, "data-replay.db"),
		},
	}
	return doc
}

func writeOperatorDocument(secretDir string, doc runtimeconfig.Document) error {
	if err := runtimeconfig.Validate(doc); err != nil {
		return fmt.Errorf("generated operator configuration is invalid: %w", err)
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode operator configuration")
	}
	path := filepath.Join(secretDir, "operator.json")
	return storage.AtomicWritePrivateFile(path, raw)
}
