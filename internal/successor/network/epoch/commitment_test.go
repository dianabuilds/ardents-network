package epoch

import (
	"reflect"
	"testing"
)

// The batched tree-once proofs must be byte-identical to the per-index proof
// and still verify against the canonical root (audit NW-1).
func TestCommitmentProofsMatchSingleProof(t *testing.T) {
	t.Parallel()
	for length := 0; length <= 64; length++ {
		values := make([][]byte, length)
		for index := range values {
			values[index] = []byte{byte(index), byte(length), byte(index * 7)}
		}
		proofs := epochCommitmentProofs(values, 0x11)
		if len(proofs) != length {
			t.Fatalf("length %d proofs=%d", length, len(proofs))
		}
		root := epochCommitmentRoot(values, 0x11)
		for index, value := range values {
			want := epochCommitmentProof(values, index, 0x11)
			if !reflect.DeepEqual(proofs[index], want) {
				t.Fatalf("length %d index %d batched proof differs from single proof", length, index)
			}
			if !verifyEpochCommitment(value, uint32(index), uint32(length), proofs[index], root) {
				t.Fatalf("length %d index %d batched proof rejected", length, index)
			}
		}
	}
}

func TestProofCoversCanonicalTreeShapes(t *testing.T) {
	t.Parallel()
	for length := 1; length <= 64; length++ {
		values := make([][]byte, length)
		for index := range values {
			values[index] = []byte{byte(index), byte(length)}
		}
		root := epochCommitmentRoot(values, 0x11)
		for index, value := range values {
			proof := epochCommitmentProof(values, index, 0x11)
			if !verifyEpochCommitment(value, uint32(index), uint32(length), proof, root) {
				t.Fatalf("length %d index %d proof rejected", length, index)
			}
		}
	}
}
