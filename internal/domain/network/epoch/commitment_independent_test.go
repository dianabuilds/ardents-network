package epoch

import (
	"encoding/hex"
	"testing"
)

func TestIndependentCommitmentRefusesForeignSibling(t *testing.T) {
	decode := func(s string) (v [32]byte) {
		b, e := hex.DecodeString(s)
		if e != nil {
			t.Fatal(e)
		}
		copy(v[:], b)
		return
	}
	root := decode("c91d808466c298187df3c71e1700099cbc8e207fcf419377989a56cbf467fda6")
	sibling := decode("4af98d9d0ff04fc04743c5adf4959bb5eccf493fafed5b4e90b921627325b318")
	if !verifyEpochCommitment([]byte("a"), 0, 2, [][32]byte{sibling}, root) {
		t.Fatal("healthy independent vector refused")
	}
	sibling[0] ^= 1
	if verifyEpochCommitment([]byte("a"), 0, 2, [][32]byte{sibling}, root) {
		t.Fatal("foreign sibling authenticated original leaf")
	}
}
