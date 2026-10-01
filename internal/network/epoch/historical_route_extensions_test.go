package epoch_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	assignmentfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/assignment"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// The interactive consumer is retired, but signed v2/v3 Epoch extensions remain
// authenticated historical input. A profile cannot claim another assigned Node.
func TestVerifyRetainsHistoricalRouteExtensionBindings(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	networkID := sha256.Sum256([]byte("historical Route extension bindings"))
	seed := sha256.Sum256([]byte("historical Route assignment"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x73}, ed25519.SeedSize))
	authorityPublic := authority.Public().(ed25519.PublicKey)
	domains := []string{epoch.DomainDestinationResolution, epoch.DomainTransitIssuance}
	selected := make(map[string]networkfixture.Record, len(domains))
	records := make([]networkfixture.Record, 0, len(domains))
	for marker := byte(1); marker < 128 && len(selected) < len(domains); marker++ {
		family := fmt.Sprintf("historical-family-%d", marker)
		domain, err := assignmentfixture.Select(networkID, 1, seed, family, domains)
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := selected[domain]; exists {
			continue
		}
		record, err := networkfixture.BuildRecord(networkfixture.RecordSpec{
			NetworkID: networkID, NodeID: sha256.Sum256([]byte{0x73, marker}), Generation: 1,
			ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(2 * time.Hour),
			Family: family, Endpoint: fmt.Sprintf("127.0.0.1:%d", 43000+int(marker)),
			Carrier: epoch.CarrierLegacyTCP, Capability: 2, Capacity: 1,
			PrivateKey: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize)),
		})
		if err != nil {
			t.Fatal(err)
		}
		selected[domain] = record
		records = append(records, record)
	}
	if len(selected) != len(domains) {
		t.Fatal("fixture did not assign both historical roles")
	}
	inputs := make([][]byte, len(records))
	for index, record := range records {
		inputs[index] = record.Raw
	}
	policy := epoch.Policy{
		NetworkID:   networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(authorityPublic): authorityPublic},
		Threshold:   1, Profile: epoch.ProfileInteractiveRoute, Now: now,
	}
	for _, version := range []byte{2, 3} {
		t.Run(fmt.Sprintf("AREP-v%d", version), func(t *testing.T) {
			spec := networkfixture.EpochSpec{
				NetworkID: networkID, Number: 1, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour),
				Inputs: inputs, Accepted: records, AssignmentSeed: seed,
				Profile: epoch.ProfileInteractiveRoute, Version: version,
				DestinationNodeID:  selected[epoch.DomainDestinationResolution].NodeID,
				DestinationProfile: []byte("signed historical Gateway profile"),
				Domains:            domains, Authorities: []ed25519.PrivateKey{authority},
			}
			if version == 3 {
				spec.TransitIssuerNodeID = selected[epoch.DomainTransitIssuance].NodeID
				spec.TransitIssuerProfile = []byte("signed historical issuer profile")
			}
			built, err := networkfixture.BuildEpoch(spec)
			if err != nil {
				t.Fatal(err)
			}
			verified, err := epoch.Verify(policy, built.Raw, built.Inputs, built.Materials[:1], true)
			if err != nil {
				t.Fatalf("verify signed historical Epoch: %v", err)
			}
			if verified.Snapshot.DestinationResolutionNodeID != spec.DestinationNodeID ||
				!bytes.Equal(verified.Snapshot.DestinationResolutionProfile[:verified.Snapshot.DestinationResolutionProfileSize], spec.DestinationProfile) {
				t.Fatal("verified Gateway binding differs from signed Epoch")
			}
			if version == 3 && (verified.Snapshot.TransitIssuanceNodeID != spec.TransitIssuerNodeID ||
				!bytes.Equal(verified.Snapshot.TransitIssuanceProfile[:verified.Snapshot.TransitIssuanceProfileSize], spec.TransitIssuerProfile)) {
				t.Fatal("verified issuer binding differs from signed Epoch")
			}
			wrongGateway := spec
			wrongGateway.DestinationNodeID = selected[epoch.DomainTransitIssuance].NodeID
			signedWrongGateway, err := networkfixture.BuildEpoch(wrongGateway)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := epoch.Verify(policy, signedWrongGateway.Raw, signedWrongGateway.Inputs, signedWrongGateway.Materials[:1], true); err == nil {
				t.Fatal("accepted a signed Gateway profile for the wrong assigned Node")
			}
			if version == 3 {
				wrongIssuer := spec
				wrongIssuer.TransitIssuerNodeID = selected[epoch.DomainDestinationResolution].NodeID
				signedWrongIssuer, err := networkfixture.BuildEpoch(wrongIssuer)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := epoch.Verify(policy, signedWrongIssuer.Raw, signedWrongIssuer.Inputs, signedWrongIssuer.Materials[:1], true); err == nil {
					t.Fatal("accepted a signed issuer profile for the wrong assigned Node")
				}
			}
		})
	}
}
