package enrollment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func TestIntroductionInventoryIndependentCanonicalOracle(t *testing.T) {
	files := map[string][]byte{}
	var pairs []string
	for _, name := range []string{"ardents-introduction.service", "ardents-node-linux-amd64", "infrastructure-introduction.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"} {
		files[name] = []byte("Introduction public:" + name)
		sum := sha256.Sum256(files[name])
		pairs = append(pairs, fmt.Sprintf("%q:%q", name, hex.EncodeToString(sum[:])))
	}
	want := []byte("{\"schema\":\"ardents-infrastructure-enrollment-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":2,\"invocation\":\"introduction\",\"files\":{" + strings.Join(pairs, ",") + "}}\n")
	facts := InfrastructureFacts{"linux-amd64", "one", 2, "introduction"}
	got, err := PrepareInfrastructureInventory(facts, files)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("independent Introduction inventory %q: %v", got, err)
	}
	observed, hashes, err := ValidateInfrastructureInventory(want, files)
	if err != nil || observed != facts || len(hashes) != 7 {
		t.Fatal("Introduction public inventory", observed, hashes, err)
	}
	for _, role := range []string{"source", "forwarding", "issuer", "join", "descriptor"} {
		if _, _, err := ValidateInfrastructureInventory([]byte(strings.Replace(string(want), "\"introduction\"", fmt.Sprintf("%q", role), 1)), files); err == nil {
			t.Fatal("foreign role inventory accepted", role)
		}
	}
	files["ardents-node-linux-amd64"] = []byte("substituted program")
	if _, _, err := ValidateInfrastructureInventory(want, files); err == nil {
		t.Fatal("substituted Introduction program accepted")
	}
	files["infrastructure-introduction.json"] = bytes.Repeat([]byte("x"), (16<<10)+1)
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("Introduction introduction bound widened")
	}
	files["infrastructure-introduction.json"] = []byte("Introduction introduction")
	files["extra"] = []byte("extra")
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("extra inventory accepted")
	}
	delete(files, "extra")
	delete(files, "ardents-introduction.service")
	files["ardents-forwarding.service"] = []byte("foreign unit")
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("Forwarding unit substituted for Introduction")
	}
}
