//go:build linux

package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func TestForwardingDescriptorIndependentBytesAndRoleSeparation(t *testing.T) {
	program, template := []byte("node program"), []byte("forwarding template")
	p, u := sha256.Sum256(program), sha256.Sum256(template)
	want := []byte(fmt.Sprintf("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":3,\"invocation\":\"forwarding\",\"files\":{\"ardents-forwarding.service\":\"%s\",\"ardents-node-linux-amd64\":\"%s\"}}\n", hex.EncodeToString(u[:]), hex.EncodeToString(p[:])))
	got, err := PrepareForwardingDescriptor("one", 3, program, template)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("independent bytes %q: %v", got, err)
	}
	if _, err := ValidateForwardingResources(want, program, template); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeDescriptor(want); err == nil {
		t.Fatal("Source accepted Forwarding descriptor")
	}
	source, err := PrepareDescriptor("one", 3, program, template)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeForwardingDescriptor(source); err == nil {
		t.Fatal("Forwarding accepted Source descriptor")
	}
	for _, mutation := range []struct{ before, after string }{
		{"\"forwarding\"", "\"join\""}, {"ardents-forwarding.service", "ardents-source.service"},
		{"\"release_version\":3", "\"release_version\":0"}, {"\"files\":", "\"extra\":0,\"files\":"},
		{"\"platform\":", "\"platform\":\"linux-amd64\",\"platform\":"},
	} {
		if _, err := DecodeForwardingDescriptor([]byte(strings.Replace(string(want), mutation.before, mutation.after, 1))); err == nil {
			t.Fatal("foreign/noncanonical descriptor accepted", mutation)
		}
	}
	for _, raw := range [][]byte{nil, append(bytes.Clone(want), '\n'), bytes.TrimSpace(want), bytes.Repeat([]byte("x"), (16<<10)+1)} {
		if _, err := DecodeForwardingDescriptor(raw); err == nil {
			t.Fatal("noncanonical descriptor accepted")
		}
	}
	for _, replacement := range []struct{ program, template []byte }{{[]byte("other"), template}, {program, []byte("other")}, {nil, template}} {
		if _, err := ValidateForwardingResources(want, replacement.program, replacement.template); err == nil {
			t.Fatal("substituted resource accepted")
		}
	}
}
