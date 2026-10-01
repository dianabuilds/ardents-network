//go:build linux

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQualificationPlanRejectsMissingAndCallerSelectedPrograms(t *testing.T) {
	for _, body := range []string{"{}", "{\"Participants\":[]}", "{\"Executable\":\"/bin/sh\"}", strings.Repeat(" ", (256<<10)+1)} {
		if _, err := decodePlan(strings.NewReader(body)); err == nil {
			t.Fatal("invalid fixed plan accepted")
		}
	}
}
func TestQualificationIdentityRequiresExactNonzeroHex(t *testing.T) {
	for _, value := range []string{"", strings.Repeat("0", 64), strings.Repeat("AB", 32), strings.Repeat("1", 63)} {
		if _, err := decodeIdentity(value); err == nil {
			t.Fatal("ambiguous identity accepted")
		}
	}
	if _, err := decodeIdentity(strings.Repeat("ab", 32)); err != nil {
		t.Fatal(err)
	}
}

func TestQualificationPlanSeparatesUserAndPublisherOwners(t *testing.T) {
	participant := qualificationTestParticipant(t)
	plan := qualificationPlan{Participants: []qualificationParticipantPlan{
		{Participant: participant, Role: 1, Profile: 1, Condition: 1, Seed: strings.Repeat("01", 32), HostingRoot: "/var/lib/ardents/hosting"},
		{Participant: participant, Role: 2, Profile: 1, Condition: 1, Seed: strings.Repeat("01", 32), HostingRoot: "/var/lib/ardents/hosting"},
	}}
	body, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodePlan(strings.NewReader(string(body))); err == nil || !strings.Contains(err.Error(), "cannot combine User and Publisher owners") {
		t.Fatalf("mixed host owner plan accepted: %v", err)
	}
}

func TestQualificationPlanRequiresCompleteOwnerInventory(t *testing.T) {
	participant := qualificationTestParticipant(t)
	plan := qualificationPlan{Participants: []qualificationParticipantPlan{{
		Participant: participant, Role: 1, Profile: 1, Condition: 1, Seed: strings.Repeat("01", 32), HostingRoot: "/var/lib/ardents/hosting",
	}}}
	body, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodePlan(strings.NewReader(string(body))); err == nil || !strings.Contains(err.Error(), "all four Readers") {
		t.Fatalf("partial User owner plan accepted: %v", err)
	}
}

func qualificationTestParticipant(t *testing.T) json.RawMessage {
	t.Helper()
	const id = "0101010101010101010101010101010101010101010101010101010101010101"
	participant := map[string]any{
		"Network": map[string]any{
			"Root": "/var/lib/ardents/state", "NetworkID": [32]byte{1}, "AcceptedProfile": "ardents-route-v3",
			"Authorities": map[string]string{id: id},
		},
		"EntryRoot": "/var/lib/ardents/entry", "LocalRoleRoot": "/var/lib/ardents/roles", "TokenRoot": "/var/lib/ardents/tokens",
		"PublicationRoot": "/var/lib/ardents/publications", "ServiceInstanceRoot": "/var/lib/ardents/instance",
		"ApplicationAddress": "/run/ardents/reader.sock", "AdministrationAddress": "/run/ardents/publisher.sock",
		"BrokerID": [32]byte{2}, "ConnectionPrincipal": [32]byte{3}, "AdministrationPrincipal": [32]byte{4},
		"ReaderPermission":    map[string]any{"RequestPath": "/run/ardents/reader.request", "ResponsePath": "/run/ardents/reader.response", "Maxima": [3]uint32{1, 1, 1}},
		"PublisherPermission": map[string]any{"RequestPath": "/run/ardents/publisher.request", "ResponsePath": "/run/ardents/publisher.response", "Maxima": [3]uint32{1, 1, 1}},
	}
	body, err := json.Marshal(participant)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
