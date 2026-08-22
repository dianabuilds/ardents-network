package namestore

import (
	"encoding/hex"
	"testing"
)

func TestNamespaceCommitmentPreservesCanonicalRootAndRejectsMutatedPath(t *testing.T) {
	values := [][]byte{[]byte("alice"), []byte("bob"), []byte("carol")}
	root := namespaceCommitmentRoot(values, emptyRecordTag)
	if got := hex.EncodeToString(root[:]); got != "161c04410ba0272be70b4f4a694229a2bc85b3e563d5bb7ed18ccbd84645524b" {
		t.Fatalf("Namespace commitment root = %s", got)
	}
	path := namespaceProof(values, 1, emptyRecordTag)
	if !verifyNamespaceProof(values[1], 1, uint32(len(values)), path, root) {
		t.Fatal("canonical Namespace commitment path was rejected")
	}
	path[0][0] ^= 1
	if verifyNamespaceProof(values[1], 1, uint32(len(values)), path, root) {
		t.Fatal("mutated Namespace commitment path was accepted")
	}
}
