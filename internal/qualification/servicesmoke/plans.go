package servicesmoke

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func writeGeneration(root string, generation int, at time.Time, credential serviceconn.Credential,
	private ed25519.PrivateKey, authority, introduction [32]byte) ([32]byte, error) {
	generationRoot := filepath.Join(root, "generations", strconv.Itoa(generation))
	if err := os.MkdirAll(generationRoot, 0o700); err != nil {
		return [32]byte{}, err
	}
	if err := byteio.WriteJSON(filepath.Join(generationRoot, "credential.json"), credential, 8<<10); err != nil {
		return [32]byte{}, err
	}
	if err := os.WriteFile(filepath.Join(generationRoot, "instance.hex"), []byte(hex.EncodeToString(private)), 0o600); err != nil {
		return [32]byte{}, err
	}
	principals := make([][32]byte, 5)
	for index := range principals {
		value, err := random32()
		if err != nil {
			return [32]byte{}, err
		}
		principals[index] = value
	}
	common := map[string]any{"NetworkID": hex32(credential.NetworkID), "AuthorityPublic": hex32(authority),
		"IntroductionPublic": hex32(introduction),
		"At":                 at.Format(time.RFC3339), "Deadline": "15s", "BytesEachDirection": 64 << 10,
		"PublicationFile": "/run/ardents/publication/current.bin"}
	publisher := clone(common)
	for key, value := range map[string]any{"Role": "publisher", "BrokerID": hex32(principals[4]),
		"ConnectionPrincipal": hex32(principals[0]), "AdministrationPrincipal": hex32(principals[1]),
		"ApplicationSocket": "/run/ardents/publisher-app/app.sock", "RouteSocket": "/run/ardents/publisher-route/route.sock",
		"AdministrationSocket": "/run/ardents/admin/admin.sock", "CredentialFile": "/run/ardents/service/credential.json",
		"InstanceKeyFile":     "/run/ardents/service/instance.hex",
		"IntroductionSocket":  "/run/ardents/introduction-ack/ack.sock",
		"GenerationStateFile": "/run/ardents/lifecycle/generation"} {
		publisher[key] = value
	}
	client := clone(common)
	for key, value := range map[string]any{"Role": "client", "BrokerID": hex32(principals[3]),
		"ConnectionPrincipal": hex32(principals[2]), "Target": hex32(credential.Target),
		"ApplicationSocket": "/run/ardents/client-app/app.sock", "RouteSocket": "/run/ardents/client-route/route.sock"} {
		client[key] = value
	}
	if err := byteio.WriteJSON(filepath.Join(generationRoot, "publisher.json"), publisher, 64<<10); err != nil {
		return [32]byte{}, err
	}
	if err := byteio.WriteJSON(filepath.Join(generationRoot, "client.json"), client, 64<<10); err != nil {
		return [32]byte{}, err
	}
	return [32]byte{}, nil
}

func clone(input map[string]any) map[string]any {
	result := make(map[string]any, len(input)+8)
	for key, value := range input {
		result[key] = value
	}
	return result
}

func credentialBytes(value serviceconn.Credential) []byte {
	raw, _ := json.Marshal(value)
	return raw
}
