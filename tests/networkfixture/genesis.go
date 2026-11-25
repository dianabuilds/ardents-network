package networkfixture

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// PrepareNetworkGenesis prepares canonical signed fixture inputs with a real
// issuance key inventory. Each test opens its own Network and Admission roots.
func PrepareNetworkGenesis(t testing.TB, dir string) (networkfixture.ClosedSpec, issuer.Plan, issuance.Inventory) {
	t.Helper()
	start := time.Now().UTC().Truncate(time.Hour)
	end := start.Add(time.Hour)
	if time.Until(end) < 2*time.Minute {
		end = end.Add(time.Hour)
	}
	_, authorityKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, issuerKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, receiverKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(authorityKey); clear(issuerKey); clear(receiverKey) })
	networkID := sha256.Sum256([]byte("new Network integration"))
	issuerID, receiverID := [32]byte{11}, [32]byte{12}
	var signer, permissionAuthority [32]byte
	copy(signer[:], issuerKey.Public().(ed25519.PublicKey))
	copy(permissionAuthority[:], authorityKey.Public().(ed25519.PublicKey))
	plan := issuer.Plan{AdmissionRoot: filepath.Join(dir, "quota"), KeyRoot: filepath.Join(dir, "keys"), ResultRoot: filepath.Join(dir, "results"),
		KeyBinding: issuance.Binding{Network: networkID, Issuer: issuerID, Signer: signer, Start: start, End: end}}
	if err := issuance.Initialize(t.Context(), plan.KeyRoot, plan.KeyBinding); err != nil {
		t.Fatal(err)
	}
	keys, err := issuance.Open(t.Context(), plan.KeyRoot, plan.KeyBinding)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := keys.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	if err := keys.Close(); err != nil {
		t.Fatal(err)
	}
	spec := networkfixture.ClosedSpec{NetworkID: networkID, Seed: [32]byte{13}, IssuanceAuthority: permissionAuthority, Number: 1,
		NotBefore: start, NotAfter: end, Authority: authorityKey, Nodes: []networkfixture.ClosedNode{
			{RecordSpec: networkfixture.RecordSpec{NodeID: issuerID, Generation: 7, ValidFrom: start, ValidUntil: end, Endpoint: "127.0.0.1:4401", Carrier: epoch.CarrierClosedTCP, Capability: 2, Capacity: 1, PrivateKey: issuerKey}, RoleDomain: 2, Subrole: 6},
			{RecordSpec: networkfixture.RecordSpec{NodeID: receiverID, Generation: 9, ValidFrom: start, ValidUntil: end, Endpoint: "127.0.0.1:4402", Carrier: epoch.CarrierClosedTCP, Capability: 2, Capacity: 1, PrivateKey: receiverKey}, RoleDomain: 3, Subrole: 1},
		}}
	for _, key := range inventory.Keys {
		spec.Keys = append(spec.Keys, networkfixture.ClosedTokenKey{WindowStart: time.Unix(int64(key.Window), 0).UTC(), Class: key.Class, SPKI: key.SPKI})
	}
	return spec, plan, inventory
}

// PrepareIssuer initializes the quota and result stores for the signed fixture.
func PrepareIssuer(t testing.TB, plan *issuer.Plan, inventory issuance.Inventory, spec networkfixture.ClosedSpec, member networkfixture.ClosedNode, profile [32]byte) {
	t.Helper()
	plan.AdmissionBinding = quota.LedgerBinding{
		Network: spec.NetworkID, Issuer: member.NodeID, Authority: spec.IssuanceAuthority,
		Profile: profile, Duty: member.Generation, Start: spec.NotBefore, End: spec.NotAfter, Keys: inventory.Keys,
	}
	if err := quota.Initialize(plan.AdmissionRoot, plan.AdmissionBinding); err != nil {
		t.Fatal(err)
	}
	if result := issuer.Initialize(t.Context(), *plan); result.Outcome != "initialized-results" {
		t.Fatal(result)
	}
}
