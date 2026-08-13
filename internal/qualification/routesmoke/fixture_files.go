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

func writeFixtureFiles(root string, now time.Time, authority ed25519.PublicKey, identities []identity,
	plan route.Plan, publisherID, manifest [32]byte) error {
	roles := []string{"initiator", "introduction", "rendezvous", "responder"}
	for index, role := range append(roles, "publisher", "client") {
		secretRoot := filepath.Join(root, "secrets", role)
		if err := os.WriteFile(filepath.Join(secretRoot, "cert.pem"), identities[index].certificate, 0o600); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(secretRoot, "key.pem"), identities[index].key, 0o600); err != nil {
			return err
		}
	}
	encodedManifest := hex.EncodeToString(manifest[:])
	for index, position := range plan.Positions {
		upstream := identities[5].public
		if index > 0 {
			upstream = identities[index-1].public
		}
		nextID, nextAddress, nextPin := publisherID, "172.31.20.16:4605", identities[4].public
		if index < 3 {
			nextID, nextAddress, nextPin = plan.Positions[index+1].NodeID, plan.Positions[index+1].Endpoint, identities[index+1].public
		}
		value := map[string]any{"Role": position.Role, "ManifestDigest": encodedManifest,
			"NetworkID": hex32(plan.NetworkID), "EpochDigest": hex32(plan.Digest), "NodeID": hex32(position.NodeID),
			"Listen": position.Endpoint, "Certificate": "/run/ardents/secrets/cert.pem", "Key": "/run/ardents/secrets/key.pem",
			"UpstreamPin": hex32(upstream), "NextNodeID": hex32(nextID), "Next": nextAddress,
			"NextPin": hex32(nextPin), "Deadline": "15s"}
		if err := byteio.WriteJSON(filepath.Join(root, "plans", position.Role+".json"), value, 64<<10); err != nil {
			return err
		}
	}
	publisher := map[string]any{"Role": "publisher", "ManifestDigest": encodedManifest,
		"NetworkID": hex32(plan.NetworkID), "EpochDigest": hex32(plan.Digest), "NodeID": hex32(publisherID),
		"Listen": "172.31.20.16:4605", "Certificate": "/run/ardents/secrets/cert.pem", "Key": "/run/ardents/secrets/key.pem",
		"UpstreamPin": hex32(identities[3].public), "ServiceCertificate": "/run/ardents/secrets/cert.pem",
		"ServiceKey": "/run/ardents/secrets/key.pem", "Deadline": "15s"}
	if err := byteio.WriteJSON(filepath.Join(root, "plans", "publisher.json"), publisher, 64<<10); err != nil {
		return err
	}
	client := map[string]any{"Role": "client", "ManifestDigest": encodedManifest, "StateRoot": "/run/ardents/state",
		"NetworkID": hex32(plan.NetworkID), "Authorities": []string{hex.EncodeToString(authority)}, "Threshold": 1,
		"At": now.Format(time.RFC3339), "Seed": hex32(plan.Seed), "Certificate": "/run/ardents/secrets/cert.pem",
		"Key": "/run/ardents/secrets/key.pem", "PublisherPin": hex32(identities[4].public), "Deadline": "15s"}
	return byteio.WriteJSON(filepath.Join(root, "plans", "client.json"), client, 64<<10)
}

func hex32(value [32]byte) string { return hex.EncodeToString(value[:]) }
