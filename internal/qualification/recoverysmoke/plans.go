package recoverysmoke

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

type grantBinding struct {
	Broker, Principal [32]byte
	Surface           string
}

const recoveryOperationLifetime = "30s"

func writeGeneration(root string, generation int, at time.Time, credential serviceconn.Credential,
	authority, introduction [32]byte) ([2]grantBinding, error) {
	generationRoot := filepath.Join(root, "generations", strconv.Itoa(generation))
	if err := os.MkdirAll(generationRoot, 0o700); err != nil {
		return [2]grantBinding{}, err
	}
	if err := byteio.WriteJSON(filepath.Join(generationRoot, "credential.json"), credential, 8<<10); err != nil {
		return [2]grantBinding{}, err
	}
	principals := make([][32]byte, 5)
	for index := range principals {
		value, err := random32()
		if err != nil {
			return [2]grantBinding{}, err
		}
		principals[index] = value
	}
	common := endpointGenerationPlan(at, credential, authority, introduction)
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
		return [2]grantBinding{}, err
	}
	if err := byteio.WriteJSON(filepath.Join(generationRoot, "client.json"), client, 64<<10); err != nil {
		return [2]grantBinding{}, err
	}
	return [2]grantBinding{{Broker: principals[3], Principal: principals[2], Surface: "connection"},
		{Broker: principals[4], Principal: principals[0], Surface: "connection"}}, nil
}

func endpointGenerationPlan(at time.Time, credential serviceconn.Credential,
	authority, introduction [32]byte) map[string]any {
	return map[string]any{"NetworkID": hex32(credential.NetworkID), "AuthorityPublic": hex32(authority),
		"IntroductionPublic": hex32(introduction),
		"At":                 at.Format(time.RFC3339), "Deadline": "15s", "Lifetime": recoveryOperationLifetime,
		"BytesEachDirection": 64 << 10,
		"PublicationFile":    "/run/ardents/publication/current.bin"}
}

func clone(input map[string]any) map[string]any {
	result := make(map[string]any, len(input)+8)
	for key, value := range input {
		result[key] = value
	}
	return result
}
