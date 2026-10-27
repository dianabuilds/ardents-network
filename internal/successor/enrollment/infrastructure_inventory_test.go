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

func TestForwardingInventoryIndependentCanonicalOracle(t *testing.T) {
	files := map[string][]byte{}
	var pairs []string
	for _, name := range []string{"ardents-forwarding.service", "ardents-node-linux-amd64", "infrastructure-forwarding.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"} {
		files[name] = []byte("forwarding-public:" + name)
		sum := sha256.Sum256(files[name])
		pairs = append(pairs, fmt.Sprintf("%q:%q", name, hex.EncodeToString(sum[:])))
	}
	want := []byte("{\"schema\":\"ardents-infrastructure-enrollment-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":2,\"invocation\":\"forwarding\",\"files\":{" + strings.Join(pairs, ",") + "}}\n")
	facts := InfrastructureFacts{"linux-amd64", "one", 2, "forwarding"}
	got, err := PrepareInfrastructureInventory(facts, files)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("independent forwarding bytes %q: %v", got, err)
	}
	if _, err := parseInfrastructureInventory(want); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct{ before, after string }{
		{"\"forwarding\"", "\"source\""}, {"\"forwarding\"", "\"issuer\""},
		{"ardents-forwarding.service", "ardents-source.service"},
		{"infrastructure-forwarding.json", "infrastructure-source.json"},
		{"\"files\":", "\"extra\":0,\"files\":"},
	} {
		if _, err := parseInfrastructureInventory([]byte(strings.Replace(string(want), mutation.before, mutation.after, 1))); err == nil {
			t.Fatal("foreign inventory accepted", mutation)
		}
	}
	delete(files, "ardents-forwarding.service")
	files["ardents-source.service"] = []byte("source")
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("Source resource substituted in Forwarding inventory")
	}
	files["ardents-forwarding.service"] = []byte("forwarding")
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("extra inventory accepted")
	}
	delete(files, "ardents-source.service")
	files["infrastructure-forwarding.json"] = bytes.Repeat([]byte("x"), (16<<10)+1)
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("descriptor bound widened")
	}
}

func TestInfrastructureInventoryIndependentCanonicalOracle(t *testing.T) {
	files := map[string][]byte{}
	var pairs []string
	for _, name := range []string{"ardents-node-linux-amd64", "ardents-source.service", "infrastructure-source.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"} {
		files[name] = []byte("independent:" + name)
		sum := sha256.Sum256(files[name])
		pairs = append(pairs, fmt.Sprintf("%q:%q", name, hex.EncodeToString(sum[:])))
	}
	want := []byte("{\"schema\":\"ardents-infrastructure-enrollment-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":1,\"invocation\":\"source\",\"files\":{" + strings.Join(pairs, ",") + "}}\n")
	got, err := PrepareInfrastructureInventory(InfrastructureFacts{"linux-amd64", "one", 1, "source"}, files)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("canonical %s %v", got, err)
	}
	if _, err := parseInfrastructureInventory(want); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{string(want) + "x", strings.Replace(string(want), "source\"", "join\"", 1), strings.Replace(string(want), "\"release_version\":1", "\"release_version\":1,\"release_version\":1", 1), strings.Replace(string(want), "enrollment-v1", "enrollment-v3", 1), strings.Replace(string(want), "\"files\":", "\"foreign\":0,\"files\":", 1)} {
		if _, err := parseInfrastructureInventory([]byte(raw)); err == nil {
			t.Fatal("accepted noncanonical inventory")
		}
	}
	files["foreign"] = []byte("x")
	if _, err := PrepareInfrastructureInventory(InfrastructureFacts{"linux-amd64", "one", 1, "source"}, files); err == nil {
		t.Fatal("extra inventory")
	}
	delete(files, "foreign")
	files["targets.json"] = bytes.Repeat([]byte("x"), (1<<20)+1)
	if _, err := PrepareInfrastructureInventory(InfrastructureFacts{"linux-amd64", "one", 1, "source"}, files); err == nil {
		t.Fatal("widened signed metadata bound")
	}
}
