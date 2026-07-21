package provision

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"

	runtimeconfig "ardents/internal/config"
	"ardents/internal/storage"
)

const (
	runtimeDataDir   = "/var/lib/ardents"
	runtimeSecretDir = "/run/ardents"
)

func operatorDocument(configured options, provisioned NodeProvision) runtimeconfig.Document {
	doc := runtimeconfig.Defaults()
	doc.Node.Name = configured.nodeName
	doc.Node.DataDir = runtimeDataDir
	doc.API.TokenFile = runtimeSecretDir + "/api-token"
	doc.API.OperatorSubject = "local-deployment-operator"
	doc.Network.ListenPort = configured.transportPort
	doc.Network.StorePath = runtimeDataDir + "/waku-store.db"
	if configured.bootstrapPeer != "" {
		doc.Network.BootstrapPeers = []string{configured.bootstrapPeer}
	}
	doc.Privacy = runtimeconfig.PrivacyConfig{
		Required: true, CapabilityStore: runtimeDataDir + "/capabilities.db",
		CapabilityStoreKeyFile: runtimeSecretDir + "/capability-store.key",
		ReplayKeyFile:          runtimeSecretDir + "/replay.key", Subject: provisioned.Subject,
		TrustedIssuers: map[string]string{
			provisioned.Issuer: base64.StdEncoding.EncodeToString(provisioned.IssuerPublic),
		},
		Discovery: runtimeconfig.PrivacyChannelConfig{
			Reference: string(provisioned.DiscoveryRef), ReplayPath: runtimeDataDir + "/discovery-replay.db",
		},
		Data: runtimeconfig.PrivacyChannelConfig{
			Reference: string(provisioned.DataRef), ReplayPath: runtimeDataDir + "/data-replay.db",
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
