package nameresolution_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"sort"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type namespaceFixture struct {
	policy  namespace.MaterializationPolicy
	signers []ed25519.PrivateKey
}

func testNamespaceFixture(network [32]byte, label string) namespaceFixture {
	value := namespaceFixture{policy: namespace.MaterializationPolicy{Network: network,
		Rule: "ardents-namespace-materialization-v1", Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: 2}}
	for index := 0; index < 3; index++ {
		seed := sha256.Sum256([]byte(label + string(rune('0'+index))))
		private := ed25519.NewKeyFromSeed(seed[:])
		public := private.Public().(ed25519.PublicKey)
		value.policy.Authorities[sha256.Sum256(public)] = public
		value.signers = append(value.signers, private)
	}
	return value
}

func (value namespaceFixture) commit(t *testing.T, store *namespace.Store, epoch uint64, signed [][]byte) {
	t.Helper()
	materialization := namespace.Epoch{Number: epoch, Digest: [32]byte{byte(epoch)}, CutoffOffset: int64(epoch),
		TransitionRoot: sha256.Sum256([]byte("transitions")), TransitionLength: uint32(len(signed)),
		RejectionRoot: sha256.Sum256([]byte("rejections"))}
	if err := store.CommitLegacy(materialization, signed, value.attest); err != nil {
		t.Fatal(err)
	}
}

func (value namespaceFixture) attest(transcript []byte) ([][32]byte, [][]byte, error) {
	type signature struct {
		id  [32]byte
		raw []byte
	}
	values := make([]signature, 2)
	for index, private := range value.signers[:2] {
		values[index] = signature{id: sha256.Sum256(private.Public().(ed25519.PublicKey)),
			raw: ed25519.Sign(private, transcript)}
	}
	sort.Slice(values, func(i, j int) bool { return bytes.Compare(values[i].id[:], values[j].id[:]) < 0 })
	ids, signatures := make([][32]byte, len(values)), make([][]byte, len(values))
	for index, item := range values {
		ids[index], signatures[index] = item.id, item.raw
	}
	return ids, signatures, nil
}

func bindNamespacePolicy(view *state.Snapshot, policy namespace.MaterializationPolicy) {
	ids := make([][32]byte, 0, len(policy.Authorities))
	for id := range policy.Authorities {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	view.EpochAuthorityCount, view.EpochThreshold = uint8(len(ids)), uint8(policy.Threshold)
	for index, id := range ids {
		view.EpochAuthorityIDs[index] = id
		copy(view.EpochAuthorityKeys[index][:], policy.Authorities[id])
	}
}
