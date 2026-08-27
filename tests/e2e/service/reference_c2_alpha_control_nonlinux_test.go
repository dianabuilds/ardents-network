//go:build !linux

package service_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

// The complete command-to-C2 composition is qualified on Linux, the selected
// first participant platform. Other platform process evidence keeps the same
// persisted-floor consumer boundary but does not represent alpha-control
// command qualification.
func stageReferenceC2AlphaCorpus(t *testing.T, _, _ string, publicationPath string, authority ed25519.PublicKey, private ed25519.PrivateKey, floorRoot string, network [32]byte, linkText string) {
	t.Helper()
	stageReferenceC2AlphaCorpusDirect(t, publicationPath, authority, private, floorRoot, network, linkText)
}

// stageReferenceC2AlphaCorpusDirect retains the persistent-floor boundary on
// platforms where the separately manifested alpha-control command is not part
// of this qualification profile.
func stageReferenceC2AlphaCorpusDirect(t *testing.T, publicationPath string, authority ed25519.PublicKey, _ ed25519.PrivateKey, floorRoot string, network [32]byte, linkText string) {
	t.Helper()
	raw, err := os.ReadFile(publicationPath)
	if err != nil {
		t.Fatal(err)
	}
	var publication struct {
		AlphaAuthorityPublic, AlphaCorpus, AlphaLink string
	}
	if err := json.Unmarshal(raw, &publication); err != nil || publication.AlphaAuthorityPublic != hex.EncodeToString(authority) || publication.AlphaLink != linkText {
		t.Fatal("C2 Publisher alpha publication is invalid")
	}
	corpusBytes, err := base64.RawStdEncoding.DecodeString(publication.AlphaCorpus)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(authority, corpusBytes)
	if err != nil || corpus.Network() != network || corpus.Cohort() != "reference-c2" {
		t.Fatal("C2 Publisher alpha corpus is invalid")
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: floorRoot, Authority: authority, Cohort: "reference-c2", Network: network})
	if err != nil {
		t.Fatal(err)
	}
	defer floor.Close()
	if err := floor.Observe(corpus); err != nil {
		t.Fatal(err)
	}
}
