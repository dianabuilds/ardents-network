package namespace_test

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/epoch"
)

func TestResolutionViewsBindGatewayAdmissionAndHideLifecycleRecord(t *testing.T) {
	t.Parallel()
	network := [32]byte{15}
	policy, signers := materializationPolicy("resolution-views", network)
	store, err := epoch.Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	epoch := testEpoch(11)
	if err := store.CommitLegacy(epoch, [][]byte{signedRecord(t, network, "alice", "resolution-views-record")},
		thresholdAttester(signers[:2])); err != nil {
		t.Fatal(err)
	}
	gate, err := admission.NewAdmission([32]byte{2}, network, epoch.Number, [32]byte{3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := namespace.OpenResolutionGateway(store, epoch.Number+1, epoch.Digest, gate); err == nil {
		t.Fatal("Gateway view accepted an admission gate from another Epoch")
	}
	gateway, err := namespace.OpenResolutionGateway(store, epoch.Number, epoch.Digest, gate)
	if err != nil || !gateway.AcceptsGateway([32]byte{2}) || gateway.AcceptsGateway([32]byte{4}) {
		t.Fatalf("Gateway view=%v accepted configured Node=%v foreign Node=%v", err,
			gateway != nil && gateway.AcceptsGateway([32]byte{2}), gateway != nil && gateway.AcceptsGateway([32]byte{4}))
	}
	if gateway.Network() != network || store.Network() != network {
		t.Fatalf("Gateway view Network=%v Store Network=%v want %v",
			gateway.Network(), store.Network(), network)
	}
	operation := [32]byte{9}
	challenge, err := gate.Issue(900_000, "resolution", operation, [32]byte{1}, 915_000, [16]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	admissionProof, _ := challenge.Solve()
	if gateway.AdmitResolution(900_000, [32]byte{4}, operation, admissionProof) {
		t.Fatal("Gateway view admitted a proof bound to another Node")
	}
	if gateway.AdmitResolution(900_000, [32]byte{2}, [32]byte{10}, admissionProof) {
		t.Fatal("Gateway view admitted a proof bound to another operation")
	}
	if !gateway.AdmitResolution(900_000, [32]byte{2}, operation, admissionProof) {
		t.Fatal("Gateway view refused the exact bound resolution proof")
	}
	if gateway.AdmitResolution(900_000, [32]byte{2}, operation, admissionProof) {
		t.Fatal("Gateway view admitted a replayed resolution proof")
	}
	proof, binding, found := gateway.LookupBinding("alice", 900_000)
	if !found || binding.Name != "alice" || binding.Target == [32]byte{} {
		t.Fatalf("Gateway lookup found=%v binding=%+v", found, binding)
	}
	verifier, err := namespace.OpenResolutionVerifier(policy, epoch.Number, epoch.Digest)
	if err != nil {
		t.Fatal(err)
	}
	verified, _, err := verifier.Verify(proof, 900_000)
	if err != nil || verified != binding {
		t.Fatalf("client binding=%+v gateway binding=%+v err=%v", verified, binding, err)
	}
	proof[len(proof)-1] ^= 1
	if _, _, err := verifier.Verify(proof, 900_000); err == nil {
		t.Fatal("client verifier accepted a changed Gateway proof")
	}
}
