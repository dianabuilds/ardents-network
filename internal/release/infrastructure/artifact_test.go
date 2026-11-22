package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func TestCanonicalDescriptorIndependentOracle(t *testing.T) {
	p, u := []byte("program"), []byte("closed public template")
	ps, us := sha256.Sum256(p), sha256.Sum256(u)
	want := []byte(fmt.Sprintf("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"release-one\",\"release_version\":7,\"invocation\":\"source\",\"files\":{\"ardents-node-linux-amd64\":\"%s\",\"ardents-source.service\":\"%s\"}}\n", hex.EncodeToString(ps[:]), hex.EncodeToString(us[:])))
	got, err := PrepareRuntimeDescriptor("source", "release-one", 7, p, u)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("canonical bytes %s %v", got, err)
	}
	if _, err := DecodeDescriptor(want); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateResources(want, p, u); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateResources(want, p, []byte("different template")); err == nil {
		t.Fatal("accepted a mismatched template")
	}
	for _, raw := range []string{string(want) + "\n", strings.TrimSpace(string(want)), strings.Replace(string(want), "\"source\"", "\"forwarding\"", 1), strings.Replace(string(want), "\"release_version\":7", "\"release_version\":0", 1), strings.Replace(string(want), "\"schema\":", "\"extra\":0,\"schema\":", 1), strings.Replace(string(want), "\"platform\":", "\"platform\":\"linux-amd64\",\"platform\":", 1), strings.Replace(string(want), "ardents-source.service", "foreign.service", 1)} {
		if _, err := DecodeDescriptor([]byte(raw)); err == nil {
			t.Fatalf("accepted noncanonical %s", raw)
		}
	}
	for _, raw := range [][]byte{nil, bytes.Repeat([]byte(" "), (16<<10)+1)} {
		if _, err := DecodeDescriptor(raw); err == nil {
			t.Fatal("accepted absent/oversized")
		}
	}
}
