package routesmoke

import (
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func writeRecoveryFixtureFiles(root string, now time.Time, authority ed25519.PublicKey, plans []route.Plan,
	identities map[[32]byte]identity, client, publisher identity, publisherID, manifest [32]byte) error {
	if err := writeRecoveryIdentity(filepath.Join(root, "secrets", "client"), client); err != nil {
		return err
	}
	if err := writeRecoveryIdentity(filepath.Join(root, "secrets", "publisher"), publisher); err != nil {
		return err
	}
	for generation, plan := range plans {
		for _, position := range plan.Positions {
			name := recoveryCandidateName(position.Role, generation)
			if err := writeRecoveryIdentity(filepath.Join(root, "secrets", name), identities[position.NodeID]); err != nil {
				return err
			}
			if err := writeRecoveryNodePlan(root, name, position, plans[0], identities, client, publisher,
				publisherID, manifest); err != nil {
				return err
			}
		}
	}
	if err := writeRecoveryEndpointRoutePlans(root, now, authority, plans[0], client, publisher,
		publisherID, manifest); err != nil {
		return err
	}
	return byteio.WriteJSON(filepath.Join(root, "manifest.json"), map[string]any{
		"schema": "ardents-h3-recovery-route-fixture-v1", "created_at": now,
		"network_id": hex32(plans[0].NetworkID), "manifest_digest": hex32(manifest)}, 64<<10)
}

func writeRecoveryIdentity(root string, value identity) error {
	if err := os.WriteFile(filepath.Join(root, "cert.pem"), value.certificate, 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "key.pem"), value.key, 0o600)
}

func writeRecoveryNodePlan(root, name string, position route.Position, initial route.Plan,
	identities map[[32]byte]identity, client, publisher identity, publisherID, manifest [32]byte) error {
	index := routePosition(position.Role)
	upstream := client.public
	if index > 0 {
		upstream = initial.Positions[index-1].PublicKey
	}
	nextID, nextAddress, nextPin := publisherID, "172.31.20.16:4605", publisher.public
	if index < len(initial.Positions)-1 {
		next := initial.Positions[index+1]
		nextID, nextAddress, nextPin = next.NodeID, next.Endpoint, next.PublicKey
	}
	value := map[string]any{"Role": position.Role, "ManifestDigest": hex32(manifest),
		"NetworkID": hex32(initial.NetworkID), "EpochDigest": hex32(initial.Digest), "NodeID": hex32(position.NodeID),
		"Listen": position.Endpoint, "Certificate": "/run/ardents/secrets/cert.pem",
		"Key": "/run/ardents/secrets/key.pem", "UpstreamPin": hex32(upstream), "NextNodeID": hex32(nextID),
		"Next": nextAddress, "NextPin": hex32(nextPin), "Deadline": "15s"}
	return byteio.WriteJSON(filepath.Join(root, "plans", name+".json"), value, 64<<10)
}

func writeRecoveryEndpointRoutePlans(root string, now time.Time, authority ed25519.PublicKey, plan route.Plan,
	client, publisher identity, publisherID, manifest [32]byte) error {
	publisherPlan := map[string]any{"Role": "publisher", "ManifestDigest": hex32(manifest),
		"NetworkID": hex32(plan.NetworkID), "EpochDigest": hex32(plan.Digest), "NodeID": hex32(publisherID),
		"Listen": "172.31.20.16:4605", "Certificate": "/run/ardents/secrets/cert.pem",
		"Key": "/run/ardents/secrets/key.pem", "UpstreamPin": hex32(plan.Positions[3].PublicKey),
		"ServiceCertificate": "/run/ardents/secrets/cert.pem", "ServiceKey": "/run/ardents/secrets/key.pem",
		"Deadline": "15s"}
	if err := byteio.WriteJSON(filepath.Join(root, "plans", "publisher.json"), publisherPlan, 64<<10); err != nil {
		return err
	}
	clientPlan := map[string]any{"Role": "client", "ManifestDigest": hex32(manifest),
		"StateRoot": "/run/ardents/state", "NetworkID": hex32(plan.NetworkID),
		"Authorities": []string{hex.EncodeToString(authority)}, "Threshold": 1, "At": now.Format(time.RFC3339),
		"Seed": hex32(plan.Seed), "Certificate": "/run/ardents/secrets/cert.pem",
		"Key": "/run/ardents/secrets/key.pem", "PublisherPin": hex32(publisher.public), "Deadline": "15s"}
	return byteio.WriteJSON(filepath.Join(root, "plans", "client.json"), clientPlan, 64<<10)
}

func routePosition(role string) int {
	for index, value := range []string{"initiator", "introduction", "rendezvous", "responder"} {
		if value == role {
			return index
		}
	}
	return -1
}

func recoveryCandidateName(role string, generation int) string {
	if generation == 0 {
		return role
	}
	return role + "-" + string(rune('2'+generation-1))
}
