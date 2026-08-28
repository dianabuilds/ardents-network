package node

import (
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestRendezvousDutyUsesOptionalSamePortLoopbackListenOverride(t *testing.T) {
	profile, snapshot := rendezvousBindFixture(t)
	snapshot.ProbeEndpoint = "203.0.113.24:48127"

	withoutOverride, err := rendezvousDuty(profile, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if withoutOverride.ListenAddress != "203.0.113.24:48127" {
		t.Fatalf("Rendezvous omitted override listen = %q", withoutOverride.ListenAddress)
	}

	profile.LoopbackListenOverride = "127.0.0.1:48127"
	withOverride, err := rendezvousDuty(profile, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if withOverride.ListenAddress != "127.0.0.1:48127" || withOverride.NetworkID != snapshot.NetworkID ||
		withOverride.EpochDigest != snapshot.Digest || withOverride.CarrierProfile != route.CarrierTCP || len(withOverride.Peers) != 2 {
		t.Fatalf("Rendezvous loopback bind changed authenticated duty facts: %+v", withOverride)
	}
}

func TestRendezvousDutyRejectsInvalidLoopbackListenOverride(t *testing.T) {
	for _, test := range []struct {
		name, override string
	}{
		{"hostname", "localhost:48127"},
		{"IPv4 unspecified", "0.0.0.0:48127"},
		{"IPv6 unspecified", "[::]:48127"},
		{"non-loopback", "198.51.100.7:48127"},
		{"zero port", "127.0.0.1:0"},
		{"out-of-range port", "127.0.0.1:65536"},
		{"different port", "127.0.0.1:48128"},
		{"IPv6 different port", "[::1]:48128"},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile, snapshot := rendezvousBindFixture(t)
			snapshot.ProbeEndpoint = "203.0.113.24:48127"
			profile.LoopbackListenOverride = test.override
			if _, err := rendezvousDuty(profile, snapshot); err == nil || !strings.Contains(err.Error(), "loopback listen override") {
				t.Fatalf("Rendezvous override %q error = %v", test.override, err)
			}
		})
	}
}

func TestRendezvousDutyAcceptsIPv6LoopbackAtStatePort(t *testing.T) {
	profile, snapshot := rendezvousBindFixture(t)
	snapshot.ProbeEndpoint = "[2001:db8::24]:48127"
	profile.LoopbackListenOverride = "[::1]:48127"
	config, err := rendezvousDuty(profile, snapshot)
	if err != nil || config.ListenAddress != "[::1]:48127" {
		t.Fatalf("IPv6 loopback override = %+v / %v", config, err)
	}
}

func TestRendezvousDutyRejectsOverrideForUnspecifiedStateEndpoint(t *testing.T) {
	profile, snapshot := rendezvousBindFixture(t)
	snapshot.ProbeEndpoint = "0.0.0.0:48127"
	profile.LoopbackListenOverride = "127.0.0.1:48127"
	if _, err := rendezvousDuty(profile, snapshot); err == nil || !strings.Contains(err.Error(), "loopback listen override") {
		t.Fatalf("unspecified State endpoint override error = %v", err)
	}
}

func rendezvousBindFixture(t *testing.T) (RendezvousProfile, dutyFacts) {
	t.Helper()
	server, serverPublic := rendezvousCertificate(t, 80, "bind-rendezvous")
	_, initiatorPublic := rendezvousCertificate(t, 81, "bind-initiator")
	_, responderPublic := rendezvousCertificate(t, 82, "bind-responder")
	now := time.Now().UTC().Truncate(time.Second)
	profile := RendezvousProfile{Certificate: server, HandshakeLimit: 2, WaitingLimit: 2, PairLimit: 1,
		PairByteLimit: 1024, AdmissionTimeout: time.Second, DrainTimeout: time.Second}
	snapshot := dutyFacts{NetworkID: [32]byte{1}, Epoch: 2, Digest: [32]byte{3}, Profile: route.Profile,
		Assignment: "rendezvous", NodeID: [32]byte{4}, NodePublicKey: serverPublic, CarrierProfile: string(route.CarrierTCP),
		EpochValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute), RecordValidUntil: now.Add(2 * time.Minute),
		CandidateCount: 2, Candidates: [64]dutyCandidate{
			{NodeID: [32]byte{5}, PublicKey: initiatorPublic, Assignment: "initiator", ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute)},
			{NodeID: [32]byte{6}, PublicKey: responderPublic, Assignment: "responder", ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute)},
		}}
	return profile, snapshot
}
