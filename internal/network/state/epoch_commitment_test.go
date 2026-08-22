package state

import "testing"

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
