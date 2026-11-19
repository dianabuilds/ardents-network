package main

import (
	"encoding/hex"
	"testing"
	"time"
)

func TestNetworkStateConfigPinsClosedProfileAuthorityOnlyWhenSpecified(t *testing.T) {
	public := make([]byte, 32)
	public[0] = 9
	raw := rawConfig{root: t.TempDir(), network: hex.EncodeToString(bytes32(1)), authorities: hex.EncodeToString(public), threshold: 1,
		at: time.Unix(1_800_000_000, 0).UTC().Format(time.RFC3339), profile: "ardents-route-v3", closedProfileAuthority: hex.EncodeToString(public)}
	config, err := raw.networkStateConfig()
	if err != nil || string(config.ClosedProfileAuthority) != string(public) {
		t.Fatalf("closed State profile pin = %x, %v", config.ClosedProfileAuthority, err)
	}
	raw.profile, raw.closedProfileAuthority = "ardents-interactive-route-v2", ""
	config, err = raw.networkStateConfig()
	if err != nil || len(config.ClosedProfileAuthority) != 0 {
		t.Fatalf("ordinary profile config = %x, %v", config.ClosedProfileAuthority, err)
	}
}

// The offline adapter must supply the State owner's fixed-mode clock pair:
// without ClockObservation the owner's clock-confidence check refuses every
// acceptance, which the refusal-only command tests never observed (#649).
func TestNetworkStateConfigSuppliesFixedModeClockObservation(t *testing.T) {
	public := make([]byte, 32)
	public[0] = 9
	at := time.Unix(1_800_000_000, 0).UTC()
	raw := rawConfig{root: t.TempDir(), network: hex.EncodeToString(bytes32(1)), authorities: hex.EncodeToString(public), threshold: 1,
		at: at.Format(time.RFC3339), profile: "ardents-route-v3", closedProfileAuthority: hex.EncodeToString(public)}
	config, err := raw.networkStateConfig()
	if err != nil || !config.Now.Equal(at) || !config.ClockObservation.Equal(at) {
		t.Fatalf("fixed-mode clock pair = Now %v / ClockObservation %v, %v", config.Now, config.ClockObservation, err)
	}
}

func bytes32(value byte) []byte {
	result := make([]byte, 32)
	result[0] = value
	return result
}
