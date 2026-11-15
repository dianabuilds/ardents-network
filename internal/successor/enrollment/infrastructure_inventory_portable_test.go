package enrollment

import (
	"bytes"
	"testing"
)

func TestInfrastructureInventoryPortableExactBytesAndPurpose(t *testing.T) {
	facts := InfrastructureFacts{Platform: "linux-amd64", ReleaseIdentity: "release", ReleaseVersion: 1, Invocation: "join"}
	files := make(map[string][]byte)
	for _, name := range infrastructureNames(facts.Invocation) {
		files[name] = []byte("bytes:" + name)
	}
	original, err := PrepareInfrastructureInventory(facts, files)
	if err != nil {
		t.Fatal(err)
	}
	decoded, hashes, err := ValidateInfrastructureInventory(original, files)
	if err != nil || decoded != facts || len(hashes) != 7 {
		t.Fatalf("valid inventory refused: %v", err)
	}
	wrong := make(map[string][]byte, len(files))
	for name, body := range files {
		wrong[name] = bytes.Clone(body)
	}
	wrong["ardents-node-linux-amd64"][0] ^= 1
	if _, _, err := ValidateInfrastructureInventory(original, wrong); err == nil {
		t.Fatal("changed executable accepted")
	}
	foreign := bytes.Replace(original, []byte(`"invocation":"join"`), []byte(`"invocation":"descriptor"`), 1)
	if _, _, err := ValidateInfrastructureInventory(foreign, files); err == nil {
		t.Fatal("foreign role accepted")
	}
	if _, _, err := ValidateInfrastructureInventory(bytes.TrimSpace(original), files); err == nil {
		t.Fatal("noncanonical bytes accepted")
	}
}
