package stage6evidence

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"sort"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type namespaceFixture struct {
	policy  namespace.MaterializationPolicy
	signers []ed25519.PrivateKey
}

func newNamespaceFixture(network [32]byte) namespaceFixture {
	value := namespaceFixture{policy: namespace.MaterializationPolicy{Network: network,
		Rule: "ardents-namespace-materialization-v1", Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: 2}}
	for index := 0; index < 3; index++ {
		private := evidenceKey("namespace-epoch-" + string(rune('0'+index)))
		public := private.Public().(ed25519.PublicKey)
		value.policy.Authorities[sha256.Sum256(public)] = public
		value.signers = append(value.signers, private)
	}
	return value
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

func (value namespaceFixture) bind(view *state.Snapshot) {
	ids := namespacePolicyIDs(value.policy)
	view.EpochAuthorityCount, view.EpochThreshold = uint8(len(ids)), uint8(value.policy.Threshold)
	for index, id := range ids {
		view.EpochAuthorityIDs[index] = id
		copy(view.EpochAuthorityKeys[index][:], value.policy.Authorities[id])
	}
}

func namespacePolicyIDs(policy namespace.MaterializationPolicy) [][32]byte {
	ids := make([][32]byte, 0, len(policy.Authorities))
	for id := range policy.Authorities {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	return ids
}

func namespaceClaimPolicyIDs(policy namespace.ClaimOrder) [][32]byte {
	ids := make([][32]byte, 0, len(policy.Authorities))
	for id := range policy.Authorities {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	return ids
}

func namespaceTransitionRoot(values [][]byte) [32]byte {
	leaves := make([][32]byte, len(values))
	for index, value := range values {
		out := make([]byte, 5+len(value))
		binary.BigEndian.PutUint32(out[1:5], uint32(len(value)))
		copy(out[5:], value)
		leaves[index] = sha256.Sum256(out)
	}
	return evidenceMerkleRoot(leaves)
}
