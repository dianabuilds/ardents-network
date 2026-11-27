package selection

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestInitialGenerationMatchesIndependentBytes(t *testing.T) {
	// These literal fragments describe the v1 wire format independently of
	// the production structs and JSON encoder.
	const zeroID = "[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]"
	const networkID = "[1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]"
	const member = `{"NodeID":` + zeroID + `,"PublicKey":` + zeroID +
		`,"FamilyID":` + zeroID + `,"RecordDigest":` + zeroID +
		`,"DutyGeneration":0,"Domain":0,"NotAfter":"0001-01-01T00:00:00Z"}`
	const domain = `{"chosen":"0001-01-01T00:00:00Z","not_after":"0001-01-01T00:00:00Z","members":[` + member + "," + member + "]}"
	want := []byte(`{"version":1,"network":` + networkID + `,"generation":1,"sets":[` +
		domain + "," + domain + "," + domain + "]}")
	digest := sha256.Sum256(want)
	name := hex.EncodeToString(digest[:])
	config, _ := closedEntryFixture(t)
	owner, err := openClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	got, err := os.ReadFile(filepath.Join(config.Root, "state-"+name))
	if err != nil || !bytes.Equal(want, got) {
		t.Fatalf("fixed v1 bytes differ: %v", err)
	}
	reopened, err := readClosedSetGeneration(config.Root, name)
	if err != nil || reopened.Version != 1 || reopened.Generation != 1 || reopened.NetworkID != [32]byte{1} {
		t.Fatalf("fixed v1 generation refused: %+v / %v", reopened, err)
	}
}
