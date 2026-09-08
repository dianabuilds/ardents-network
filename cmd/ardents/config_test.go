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

func bytes32(value byte) []byte {
	result := make([]byte, 32)
	result[0] = value
	return result
}
