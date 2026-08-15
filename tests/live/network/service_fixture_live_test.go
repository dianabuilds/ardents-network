//go:build live

package network_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

const liveDirectBytes = uint32(256 << 20)
const liveDirectDuration = 60 * time.Second

type liveServiceFixture struct {
	liveFixture
	target [32]byte
}

func newLiveServiceFixture(t *testing.T, base liveFixture, direction string, transferBytes uint32) liveServiceFixture {
	t.Helper()
	now := base.now
	authorityPublic, authorityPrivate := deterministicLiveKey(0xb1)
	introductionPublic, introductionPrivate := deterministicLiveKey(0xb2)
	instancePublic, instancePrivate := deterministicLiveKey(0xb3)
	var instance [32]byte
	copy(instance[:], instancePublic)
	credential, err := (serviceconn.Credential{InstancePublic: instance, Generation: 1,
		NotBefore: now.Add(-time.Minute).Unix(), NotAfter: now.Add(35 * time.Minute).Unix(),
		NetworkID: base.network, Capabilities: 3}).Issue(authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	writeLivePlan(t, base.root, "credential", credential)
	writeLiveFile(t, filepath.Join(base.root, "instance.hex"), []byte(hex.EncodeToString(instancePrivate)))
	writeLiveFile(t, filepath.Join(base.root, "secrets", "introduction-ack.hex"), []byte(hex.EncodeToString(introductionPrivate)))
	writeLiveFile(t, filepath.Join(base.root, "client-seed.hex"), []byte(hex.EncodeToString(bytes.Repeat([]byte{17}, 32))))
	writeLiveFile(t, filepath.Join(base.root, "publisher-seed.hex"), []byte(hex.EncodeToString(bytes.Repeat([]byte{91}, 32))))

	candidateView := sha256.Sum256([]byte("live-service-candidate-view"))
	isolation := sha256.Sum256([]byte("live-service-isolation"))
	destination := sha256.Sum256([]byte("live-service-destination"))
	common := map[string]any{
		"NetworkID": liveHex(base.network), "AuthorityPublic": hex.EncodeToString(authorityPublic),
		"IntroductionPublic": hex.EncodeToString(introductionPublic), "At": now.Format(time.RFC3339),
		"Deadline": "15s", "Lifetime": "30m", "SendBytes": transferBytes, "ReceiveBytes": 0,
		"PublicationFile": "/run/ardents/publication/current.bin", "CandidateView": liveHex(candidateView),
		"IsolationContext": liveHex(isolation), "DestinationBinding": liveHex(destination),
		"RouteProfile": "h3-route-tracer-v1", "WorkSafetyNotAfter": now.Add(32 * time.Minute).Unix(),
		"WorkSafetyMaximum": now.Add(32 * time.Minute).Unix(), "NoNewRecoveryAfter": now.Add(32 * time.Minute).Unix(),
	}
	clientBroker := sha256.Sum256([]byte("live-service-client-broker"))
	clientPrincipal := sha256.Sum256([]byte("live-service-client-principal"))
	publisherBroker := sha256.Sum256([]byte("live-service-publisher-broker"))
	publisherPrincipal := sha256.Sum256([]byte("live-service-publisher-principal"))
	administrator := sha256.Sum256([]byte("live-service-administrator"))
	client := cloneLivePlan(common)
	client["Role"], client["BrokerID"], client["ConnectionPrincipal"] = "client", liveHex(clientBroker), liveHex(clientPrincipal)
	client["Target"], client["ApplicationSocket"] = liveHex(credential.Target), "/run/ardents/client-app/app.sock"
	client["RouteSocket"] = "/run/ardents/client-route/route.sock"
	publisher := cloneLivePlan(common)
	publisher["Role"], publisher["BrokerID"] = "publisher", liveHex(publisherBroker)
	publisher["ConnectionPrincipal"], publisher["AdministrationPrincipal"] = liveHex(publisherPrincipal), liveHex(administrator)
	publisher["ApplicationSocket"], publisher["RouteSocket"] = "/run/ardents/publisher-app/app.sock", "/run/ardents/publisher-route/route.sock"
	publisher["AdministrationSocket"], publisher["IntroductionSocket"] = "/run/ardents/admin/admin.sock", "/run/ardents/introduction-ack/ack.sock"
	publisher["CredentialFile"], publisher["InstanceKeyFile"] = "/run/ardents/input/credential.json", "/run/ardents/input/instance.hex"
	publisher["GenerationStateFile"] = "/run/ardents/lifecycle/generation"
	if direction == "client-to-publisher" {
		publisher["SendBytes"], publisher["ReceiveBytes"] = 0, transferBytes
	} else if direction == "publisher-to-client" {
		client["SendBytes"], client["ReceiveBytes"] = 0, transferBytes
		publisher["SendBytes"], publisher["ReceiveBytes"] = transferBytes, 0
	} else {
		t.Fatalf("invalid live transfer direction %q", direction)
	}
	writeLivePlan(t, base.root, "service-client", client)
	writeLivePlan(t, base.root, "service-publisher", publisher)

	base.writeServiceRoutePlans(t)
	return liveServiceFixture{liveFixture: base, target: credential.Target}
}

func (value liveFixture) writeServiceRoutePlans(t *testing.T) {
	t.Helper()
	plans := filepath.Join(value.root, "plans")
	writeLivePlan(t, plans, "publisher", map[string]any{
		"Role": "publisher", "ManifestDigest": liveHex(value.manifest), "NetworkID": liveHex(value.network),
		"EpochDigest": liveHex(value.epochDigest), "NodeID": liveHex(value.publisherID), "Listen": value.addresses[4],
		"Certificate": value.identities[4].cert, "Key": value.identities[4].key,
		"UpstreamPin": liveHex(value.identities[3].public), "RawAttachment": true,
		"Stream": "/run/ardents/publisher-route/route.sock", "Deadline": "15s", "Lifetime": "30m",
		"MaximumAttachments": 4, "AttachmentTarget": 1, "ResourceProfile": "h3-np1-v1",
	})
	for index, position := range value.plan.Positions {
		upstream := value.identities[5].public
		if index > 0 {
			upstream = value.identities[index-1].public
		}
		nextID, nextAddress, nextPin := value.publisherID, value.addresses[4], value.identities[4].public
		if index < 3 {
			nextID, nextAddress, nextPin = value.plan.Positions[index+1].NodeID, value.addresses[index+1], value.identities[index+1].public
		}
		plan := map[string]any{
			"Role": position.Role, "ManifestDigest": liveHex(value.manifest), "NetworkID": liveHex(value.network),
			"EpochDigest": liveHex(value.epochDigest), "NodeID": liveHex(position.NodeID), "Listen": value.addresses[index],
			"Certificate": value.identities[index].cert, "Key": value.identities[index].key,
			"UpstreamPin": liveHex(upstream), "NextNodeID": liveHex(nextID), "Next": nextAddress,
			"NextPin": liveHex(nextPin), "Deadline": "15s", "Lifetime": "30m",
			"MaximumAttachments": 4, "AttachmentTarget": 1, "ResourceProfile": "h3-np1-v1",
		}
		if position.Role == "introduction" {
			plan["AcknowledgementSocket"] = "/run/ardents/introduction-ack/ack.sock"
			plan["AcknowledgementKey"] = "/run/ardents/secrets/introduction-ack.hex"
		}
		writeLivePlan(t, plans, position.Role, plan)
	}
	authority := value.authority.Public().(ed25519.PublicKey)
	writeLivePlan(t, plans, "client", map[string]any{
		"Role": "client", "ManifestDigest": liveHex(value.manifest), "StateRoot": "/run/ardents/state",
		"NetworkID": liveHex(value.network), "Authorities": []string{hex.EncodeToString(authority)}, "Threshold": 1,
		"At": value.now.Format(time.RFC3339), "Seed": liveHex(value.plan.Seed),
		"Certificate": value.identities[5].cert, "Key": value.identities[5].key,
		"RawAttachment": true, "Stream": "/run/ardents/client-route/route.sock", "Deadline": "15s", "Lifetime": "30m",
	})
}

func deterministicLiveKey(marker byte) (ed25519.PublicKey, ed25519.PrivateKey) {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	return private.Public().(ed25519.PublicKey), private
}

func cloneLivePlan(input map[string]any) map[string]any {
	result := make(map[string]any, len(input)+8)
	for key, value := range input {
		result[key] = value
	}
	return result
}
