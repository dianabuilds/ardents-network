package service_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

type recoveryProcessFixture struct {
	root, clientPlan, publisherPlan                          string
	clientApplication, publisherApplication                  string
	clientRoute, publisherRoute, administration, publication string
	clientSeed, publisherSeed                                string
	target                                                   [32]byte
	introductionPrivate                                      ed25519.PrivateKey
}

func newRecoveryProcessFixture(t *testing.T) recoveryProcessFixture {
	t.Helper()
	root, err := os.MkdirTemp("", "ardents-recovery-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove recovery fixture: %v", err)
		}
	})
	path := func(name string) string { return filepath.Join(root, name) }
	now := time.Now().UTC().Truncate(time.Second)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	introductionPublic, introductionPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	instancePublic, instancePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	network := sha256.Sum256([]byte("service-recovery-e2e-network"))
	var instance [32]byte
	copy(instance[:], instancePublic)
	credential, err := (serviceconn.Credential{InstancePublic: instance, Generation: 1,
		NotBefore: now.Add(-time.Minute).Unix(), NotAfter: now.Add(10 * time.Minute).Unix(),
		NetworkID: network, Capabilities: 3}).Issue(authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	credentialPath, instancePath := path("credential.json"), path("instance.hex")
	writeJSONFixture(t, credentialPath, credential)
	writeFixture(t, instancePath, []byte(hex.EncodeToString(instancePrivate)))

	clientBroker := sha256.Sum256([]byte("service-recovery-client-broker"))
	clientPrincipal := sha256.Sum256([]byte("service-recovery-client-principal"))
	publisherBroker := sha256.Sum256([]byte("service-recovery-publisher-broker"))
	publisherPrincipal := sha256.Sum256([]byte("service-recovery-publisher-principal"))
	administrator := sha256.Sum256([]byte("service-recovery-administrator"))
	candidateView := sha256.Sum256([]byte("service-recovery-candidate-view"))
	isolation := sha256.Sum256([]byte("service-recovery-isolation"))
	destination := sha256.Sum256([]byte("service-recovery-destination"))
	common := map[string]any{
		"NetworkID": hex32Fixture(network), "AuthorityPublic": hex.EncodeToString(authorityPublic),
		"IntroductionPublic": hex.EncodeToString(introductionPublic), "At": now.Format(time.RFC3339),
		"Deadline": "5s", "Lifetime": "25s", "SendBytes": 8 << 20, "ReceiveBytes": 0,
		"PublicationFile": path("publication.bin"), "CandidateView": hex32Fixture(candidateView),
		"IsolationContext": hex32Fixture(isolation), "DestinationBinding": hex32Fixture(destination),
		"RouteProfile": "h3-route-tracer-v1", "WorkSafetyNotAfter": now.Add(30 * time.Second).Unix(),
		"WorkSafetyMaximum": now.Add(30 * time.Second).Unix(), "NoNewRecoveryAfter": now.Add(30 * time.Second).Unix(),
	}
	client := cloneFixture(common)
	client["Role"], client["BrokerID"], client["ConnectionPrincipal"] = "client", hex32Fixture(clientBroker), hex32Fixture(clientPrincipal)
	client["Target"], client["ApplicationSocket"], client["RouteSocket"] = hex32Fixture(credential.Target), path("client-app.sock"), path("client-route.sock")
	publisher := cloneFixture(common)
	publisher["Role"], publisher["BrokerID"], publisher["ConnectionPrincipal"] = "publisher", hex32Fixture(publisherBroker), hex32Fixture(publisherPrincipal)
	publisher["AdministrationPrincipal"], publisher["ApplicationSocket"] = hex32Fixture(administrator), path("publisher-app.sock")
	publisher["RouteSocket"], publisher["AdministrationSocket"] = path("publisher-route.sock"), path("administration.sock")
	publisher["IntroductionSocket"], publisher["CredentialFile"] = path("introduction.sock"), credentialPath
	publisher["InstanceKeyFile"], publisher["GenerationStateFile"] = instancePath, path("generation.state")
	publisher["SendBytes"], publisher["ReceiveBytes"] = 0, 8<<20

	value := recoveryProcessFixture{root: root, clientPlan: path("client.json"), publisherPlan: path("publisher.json"),
		clientApplication: path("client-app.sock"), publisherApplication: path("publisher-app.sock"),
		clientRoute: path("client-route.sock"), publisherRoute: path("publisher-route.sock"),
		administration: path("administration.sock"), publication: path("publication.bin"),
		clientSeed: path("client-seed.hex"), publisherSeed: path("publisher-seed.hex"),
		target: credential.Target, introductionPrivate: introductionPrivate}
	writeJSONFixture(t, value.clientPlan, client)
	writeJSONFixture(t, value.publisherPlan, publisher)
	writeFixture(t, value.clientSeed, []byte(hex.EncodeToString(bytes32Fixture(17))))
	writeFixture(t, value.publisherSeed, []byte(hex.EncodeToString(bytes32Fixture(91))))
	startIntroductionFixture(t, path("introduction.sock"), introductionPrivate)
	return value
}

func startIntroductionFixture(t *testing.T, path string, private ed25519.PrivateKey) {
	t.Helper()
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close(); _ = os.Remove(path) })
	result := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			result <- acceptErr
			return
		}
		defer connection.Close()
		body := make([]byte, 149)
		_, readErr := io.ReadFull(connection, body)
		if readErr == nil {
			message := append([]byte("ardents-h3-introduction-ack-v1\x00"), body...)
			_, readErr = connection.Write(ed25519.Sign(private, message))
		}
		result <- readErr
	}()
	t.Cleanup(func() {
		select {
		case err := <-result:
			if err != nil {
				t.Errorf("Introduction fixture failed: %v", err)
			}
		default:
		}
	})
}

func cloneFixture(input map[string]any) map[string]any {
	result := make(map[string]any, len(input)+8)
	for key, value := range input {
		result[key] = value
	}
	return result
}

func writeJSONFixture(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, path, raw)
}

func writeFixture(t *testing.T, path string, value []byte) {
	t.Helper()
	if err := os.WriteFile(path, value, 0o600); err != nil {
		t.Fatal(err)
	}
}

func hex32Fixture(value [32]byte) string { return hex.EncodeToString(value[:]) }

func bytes32Fixture(marker byte) []byte {
	value := make([]byte, 32)
	for index := range value {
		value[index] = marker
	}
	return value
}
