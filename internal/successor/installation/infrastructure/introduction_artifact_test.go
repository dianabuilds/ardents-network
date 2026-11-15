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

func TestIntroductionArtifactIndependentBytesAndRoleSeparation(t *testing.T) {
	program, template := []byte("Introduction program"), []byte("Introduction unit")
	p, u := sha256.Sum256(program), sha256.Sum256(template)
	want := []byte(fmt.Sprintf("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":3,\"invocation\":\"introduction\",\"files\":{\"ardents-introduction.service\":\"%s\",\"ardents-node-linux-amd64\":\"%s\"}}\n", hex.EncodeToString(u[:]), hex.EncodeToString(p[:])))
	got, err := PrepareIntroductionArtifact("one", 3, program, template)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("independent Introduction bytes %q: %v", got, err)
	}
	if _, err := ValidateIntroductionResources(want, program, template); err != nil {
		t.Fatal(err)
	}
	for _, decode := range []func([]byte) (Declaration, error){DecodeDescriptor, DecodeForwardingDescriptor, DecodeJoinDescriptor, DecodeDescriptorArtifact} {
		if _, err := decode(want); err == nil {
			t.Fatal("another invocation accepted Introduction")
		}
	}
	for _, prepare := range []func(string, int64, []byte, []byte) ([]byte, error){PrepareDescriptor, PrepareForwardingDescriptor, PrepareJoinDescriptor, PrepareDescriptorArtifact} {
		foreign, err := prepare("one", 3, program, template)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeIntroductionArtifact(foreign); err == nil {
			t.Fatal("Introduction accepted another invocation")
		}
	}
	for _, mutation := range []struct{ before, after string }{
		{"\"introduction\"", "\"issuer\""}, {"ardents-introduction.service", "ardents-forwarding.service"},
		{"\"release_version\":3", "\"release_version\":0"}, {"\"files\":", "\"extra\":0,\"files\":"},
		{"\"platform\":", "\"platform\":\"linux-amd64\",\"platform\":"},
	} {
		if _, err := DecodeIntroductionArtifact([]byte(strings.Replace(string(want), mutation.before, mutation.after, 1))); err == nil {
			t.Fatal("foreign or noncanonical Introduction accepted", mutation)
		}
	}
	for _, raw := range [][]byte{nil, append(bytes.Clone(want), '\n'), bytes.TrimSpace(want), bytes.Repeat([]byte("x"), (16<<10)+1)} {
		if _, err := DecodeIntroductionArtifact(raw); err == nil {
			t.Fatal("noncanonical Introduction accepted")
		}
	}
	for _, substituted := range []struct{ program, template []byte }{{[]byte("other"), template}, {program, []byte("other")}, {nil, template}, {program, nil}} {
		if _, err := ValidateIntroductionResources(want, substituted.program, substituted.template); err == nil {
			t.Fatal("changed Introduction resource accepted")
		}
	}
}
