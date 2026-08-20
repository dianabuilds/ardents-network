package stage6evidence

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/namelease"
	"github.com/dianabuilds/ardents-network/internal/namestore"
)

func deepNamespaceEvidence(materialization namespaceFixture, now time.Time) (string, []byte, [][]byte, error) {
	network := [32]byte{9}
	authority := evidenceKey("deep-name-authority")
	encodedAuthority := hex.EncodeToString(authority.Public().(ed25519.PublicKey))
	records := make([][]byte, 127)
	for depth := 1; depth <= len(records); depth++ {
		name := strings.Repeat("a.", depth-1) + "a"
		record := namelease.Record{Name: name, Generation: 1, Revision: 1, Lease: "active",
			Consistency: "current", Recovery: "stable", Authority: encodedAuthority,
			LeaseExpiresAt: now.Add(time.Hour).Unix(), GraceExpiresAt: now.Add(2 * time.Hour).Unix(), Continuity: 1}
		if depth > 1 {
			record.ParentName = strings.Repeat("a.", depth-2) + "a"
			record.ParentGeneration = 1
		}
		if depth == len(records) {
			record.Target = [32]byte{1}
		}
		var err error
		records[depth-1], err = nameauthority.SignRecord(network, record, authority)
		if err != nil {
			return "", nil, nil, err
		}
	}
	root, err := os.MkdirTemp("", "ardents-stage6-deep-")
	if err != nil {
		return "", nil, nil, err
	}
	defer os.RemoveAll(root)
	store, err := namestore.Open(root, materialization.policy)
	if err != nil {
		return "", nil, nil, err
	}
	defer store.Close()
	epoch := namestore.Epoch{Number: 1, Digest: [32]byte{1}, CutoffOffset: 10_000,
		TransitionRoot: namespaceTransitionRoot(records), TransitionLength: uint32(len(records)),
		RejectionRoot: sha256.Sum256([]byte("ardents-stage6-deep-no-rejections"))}
	if err = store.Commit(epoch, records, materialization.attest); err != nil {
		return "", nil, nil, err
	}
	name := strings.Repeat("a.", 126) + "a"
	proof, err := store.Lookup(name, 1)
	return name, proof, records, err
}
