package endpoint

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadEnrollmentV5ArtifactsRequiresCompleteOriginalPair(t *testing.T) {
	directory := t.TempDir()
	programPath, custodyPath := filepath.Join(directory, "ardents-linux-amd64"), filepath.Join(directory, "ardents-custody-linux-amd64")
	if _, err := ReadEnrollmentV5Artifacts("relative"); err == nil {
		t.Fatal("relative artifact directory admitted")
	}
	if err := os.WriteFile(programPath, []byte("original executable"), 0600); err != nil {
		t.Fatal(err)
	}
	if pair, err := ReadEnrollmentV5Artifacts(directory); !errors.Is(err, os.ErrNotExist) || pair.Program != nil {
		t.Fatalf("partial pair = %+v, %v", pair, err)
	}
	if err := os.WriteFile(custodyPath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadEnrollmentV5Artifacts(directory); err == nil {
		t.Fatal("empty custody artifact admitted")
	}
	if err := os.WriteFile(custodyPath, []byte("original custody"), 0600); err != nil {
		t.Fatal(err)
	}
	pair, err := ReadEnrollmentV5Artifacts(directory)
	if err != nil || string(pair.Program) != "original executable" || string(pair.Custody) != "original custody" {
		t.Fatalf("original pair = %+v, %v", pair, err)
	}
}
