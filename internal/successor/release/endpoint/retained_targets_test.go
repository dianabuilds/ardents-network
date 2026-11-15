package endpoint

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func TestRetainedTargetsRequireFreshProofAndExactIdentity(t *testing.T) {
	reference := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	decision := release.Decision{Path: "program", Digest: bytes.Repeat([]byte{1}, 32), Length: 123,
		ReleaseIdentity: "release", ReleaseVersion: 3, Platform: "linux-amd64", Architecture: "amd64", Environment: "closed", Network: "network",
		ReferenceTime: reference, Floors: release.FloorSet{TargetsVersion: 4, TargetsDigest: bytes.Repeat([]byte{2}, 32)}}
	old := RetainedTarget{Path: decision.Path, Digest: hex.EncodeToString(decision.Digest), Length: decision.Length,
		ReleaseIdentity: decision.ReleaseIdentity, ReleaseVersion: decision.ReleaseVersion, Platform: decision.Platform, Architecture: decision.Architecture,
		Environment: decision.Environment, Network: decision.Network, ReferenceTime: reference.Format(time.RFC3339Nano),
		TargetsVersion: decision.Floors.TargetsVersion, TargetsDigest: hex.EncodeToString(decision.Floors.TargetsDigest)}
	if !matchesRetainedTarget(old, decision, reference) {
		t.Fatal("matching detached observation refused")
	}
	for name, change := range map[string]func(*RetainedTarget){
		"artifact":         func(v *RetainedTarget) { v.Digest = hex.EncodeToString(bytes.Repeat([]byte{3}, 32)) },
		"length":           func(v *RetainedTarget) { v.Length++ },
		"release":          func(v *RetainedTarget) { v.ReleaseVersion++ },
		"network":          func(v *RetainedTarget) { v.Network = "foreign" },
		"floor rollback":   func(v *RetainedTarget) { v.TargetsVersion++ },
		"floor conflict":   func(v *RetainedTarget) { v.TargetsDigest = hex.EncodeToString(bytes.Repeat([]byte{4}, 32)) },
		"future reference": func(v *RetainedTarget) { v.ReferenceTime = reference.Add(time.Second).Format(time.RFC3339Nano) },
	} {
		changed := old
		change(&changed)
		if matchesRetainedTarget(changed, decision, reference) {
			t.Fatal("changed retained target accepted", name)
		}
	}
	newer := decision
	newer.Floors.TargetsVersion++
	newer.Floors.TargetsDigest = bytes.Repeat([]byte{5}, 32)
	if !matchesRetainedTarget(old, newer, reference) {
		t.Fatal("newer authenticated metadata floor refused")
	}
	if err := (Authorization{}).MatchRetainedTargets(old, old, reference); !errors.Is(err, ErrAuthorization) {
		t.Fatalf("detached observations minted fresh proofs: %v", err)
	}
}
