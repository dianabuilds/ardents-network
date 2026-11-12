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

func TestJoinDescriptorIndependentBytesAndRoleSeparation(t *testing.T) {
	program, template := []byte("JOIN program"), []byte("JOIN unit")
	p, u := sha256.Sum256(program), sha256.Sum256(template)
	want := []byte(fmt.Sprintf("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":3,\"invocation\":\"join\",\"files\":{\"ardents-join.service\":\"%s\",\"ardents-node-linux-amd64\":\"%s\"}}\n", hex.EncodeToString(u[:]), hex.EncodeToString(p[:])))
	got, err := PrepareJoinDescriptor("one", 3, program, template)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("independent JOIN bytes %q: %v", got, err)
	}
	if _, err := ValidateJoinResources(want, program, template); err != nil {
		t.Fatal(err)
	}
	for _, decode := range []func([]byte) (Declaration, error){DecodeDescriptor, DecodeForwardingDescriptor} {
		if _, err := decode(want); err == nil {
			t.Fatal("another invocation accepted JOIN")
		}
	}
	for _, prepare := range []func(string, int64, []byte, []byte) ([]byte, error){PrepareDescriptor, PrepareForwardingDescriptor} {
		foreign, err := prepare("one", 3, program, template)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeJoinDescriptor(foreign); err == nil {
			t.Fatal("JOIN accepted another invocation")
		}
	}
	for _, mutation := range []struct{ before, after string }{
		{"\"join\"", "\"issuer\""}, {"ardents-join.service", "ardents-forwarding.service"},
		{"\"release_version\":3", "\"release_version\":0"}, {"\"files\":", "\"extra\":0,\"files\":"},
		{"\"platform\":", "\"platform\":\"linux-amd64\",\"platform\":"},
	} {
		if _, err := DecodeJoinDescriptor([]byte(strings.Replace(string(want), mutation.before, mutation.after, 1))); err == nil {
			t.Fatal("foreign or noncanonical JOIN accepted", mutation)
		}
	}
	for _, raw := range [][]byte{nil, append(bytes.Clone(want), '\n'), bytes.TrimSpace(want), bytes.Repeat([]byte("x"), (16<<10)+1)} {
		if _, err := DecodeJoinDescriptor(raw); err == nil {
			t.Fatal("noncanonical JOIN accepted")
		}
	}
	for _, substituted := range []struct{ program, template []byte }{{[]byte("other"), template}, {program, []byte("other")}, {nil, template}, {program, nil}} {
		if _, err := ValidateJoinResources(want, substituted.program, substituted.template); err == nil {
			t.Fatal("changed JOIN resource accepted")
		}
	}
}
