package node

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/probe"
)

func TestAdmissionRequiresEveryPrerequisite(t *testing.T) {
	t.Parallel()
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_000_000, 0).UTC()
	config := runtimeConfig{Config: Config{NetworkID: [32]byte{1}, NodeID: [32]byte{2}, IdentityKey: private,
		Probe:          probe.Config{ListenAddress: "127.0.0.1:4101", MaximumDuty: time.Second},
		CheckPlacement: func() error { return nil }}, now: func() time.Time { return now }}
	snapshot := Facts{NetworkID: config.NetworkID, NodeID: config.NodeID, RecordPresent: true,
		EpochValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), RecordValidFrom: now.Add(-time.Hour),
		RecordValidUntil: now.Add(time.Hour), Profile: "h3-role-probe-v1", Assignment: "domain-a",
		ProbeEndpoint: config.Probe.ListenAddress, ProbeCapacity: 1, Fresh: true}
	copy(snapshot.NodePublicKey[:], public)
	if got := assessAdmission(config, snapshot); got.kind != admissionReady {
		t.Fatalf("complete admission = %+v", got)
	}
	tests := []struct {
		name string
		edit func(*Facts, *runtimeConfig)
		want admissionKind
	}{
		{"record", func(s *Facts, _ *runtimeConfig) { s.RecordPresent = false }, admissionAbsent},
		{"identity", func(s *Facts, _ *runtimeConfig) { s.NodeID[0]++ }, admissionAbsent},
		{"key", func(s *Facts, _ *runtimeConfig) { s.NodePublicKey[0]++ }, admissionFailed},
		{"network", func(s *Facts, _ *runtimeConfig) { s.NetworkID[0]++ }, admissionFailed},
		{"profile", func(s *Facts, _ *runtimeConfig) { s.Profile = "other" }, admissionPrepared},
		{"assignment", func(s *Facts, _ *runtimeConfig) { s.Assignment = "" }, admissionPrepared},
		{"capacity", func(s *Facts, _ *runtimeConfig) { s.ProbeCapacity = 0 }, admissionPrepared},
		{"endpoint", func(s *Facts, _ *runtimeConfig) { s.ProbeEndpoint = "127.0.0.1:9" }, admissionFailed},
		{"conflict", func(s *Facts, _ *runtimeConfig) { s.Conflicting = true }, admissionPrepared},
		{"freshness", func(s *Facts, _ *runtimeConfig) { s.Fresh = false }, admissionPrepared},
		{"epoch boundary", func(s *Facts, _ *runtimeConfig) { s.ValidUntil = now.Add(time.Second) }, admissionPrepared},
		{"record boundary", func(s *Facts, _ *runtimeConfig) { s.RecordValidUntil = now.Add(time.Second) }, admissionPrepared},
		{"placement", func(_ *Facts, c *runtimeConfig) {
			c.CheckPlacement = func() error { return errors.New("pressure") }
		}, admissionPrepared},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate, candidateConfig := snapshot, config
			test.edit(&candidate, &candidateConfig)
			if got := assessAdmission(candidateConfig, candidate); got.kind != test.want {
				t.Fatalf("admission = %+v, want %v", got, test.want)
			}
		})
	}
}
