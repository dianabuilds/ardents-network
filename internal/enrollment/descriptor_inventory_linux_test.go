//go:build linux

package enrollment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func TestDescriptorInventoryIndependentCanonicalOracle(t *testing.T) {
	files := map[string][]byte{}
	var pairs []string
	for _, name := range []string{"ardents-descriptor.service", "ardents-node-linux-amd64", "infrastructure-descriptor.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"} {
		files[name] = []byte("Descriptor public:" + name)
		sum := sha256.Sum256(files[name])
		pairs = append(pairs, fmt.Sprintf("%q:%q", name, hex.EncodeToString(sum[:])))
	}
	want := []byte("{\"schema\":\"ardents-infrastructure-enrollment-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":2,\"invocation\":\"descriptor\",\"files\":{" + strings.Join(pairs, ",") + "}}\n")
	facts := InfrastructureFacts{"linux-amd64", "one", 2, "descriptor"}
	got, err := PrepareInfrastructureInventory(facts, files)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("independent Descriptor inventory %q: %v", got, err)
	}
	observed, hashes, err := ValidateInfrastructureInventory(want, files)
	if err != nil || observed != facts || len(hashes) != 7 {
		t.Fatal("Descriptor public inventory", observed, hashes, err)
	}
	for _, role := range []string{"source", "forwarding", "issuer", "join", "introduction"} {
		if _, _, err := ValidateInfrastructureInventory([]byte(strings.Replace(string(want), "\"descriptor\"", fmt.Sprintf("%q", role), 1)), files); err == nil {
			t.Fatal("foreign role inventory accepted", role)
		}
	}
	files["ardents-node-linux-amd64"] = []byte("substituted program")
	if _, _, err := ValidateInfrastructureInventory(want, files); err == nil {
		t.Fatal("substituted Descriptor program accepted")
	}
	files["infrastructure-descriptor.json"] = bytes.Repeat([]byte("x"), (16<<10)+1)
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("Descriptor descriptor bound widened")
	}
	files["infrastructure-descriptor.json"] = []byte("Descriptor descriptor")
	files["extra"] = []byte("extra")
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("extra inventory accepted")
	}
	delete(files, "extra")
	delete(files, "ardents-descriptor.service")
	files["ardents-forwarding.service"] = []byte("foreign unit")
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("Forwarding unit substituted for Descriptor")
	}
}
