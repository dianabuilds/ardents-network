//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// The privileged pre-start check binds the exact launch declaration digest before
// any file, account, manager or Release observation. A wrong pin refuses before
// effects and yields no output; it never reaches the runtime or an accepting receipt.
func TestEndpointPlatformCheckRefusesWrongPinBeforeEffects(t *testing.T) {
	launch := endpointLaunchFixture()
	raw, err := json.Marshal(launch)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = executeEndpointPlatformCheck(t.Context(), []string{"check", "/etc/ardents/endpoint.json", strings.Repeat("0", 64)}, &out, &platformFiles{}, launch, raw)
	if err == nil || out.Len() != 0 {
		t.Fatalf("wrong pin reached effects: err=%v output=%q", err, out.String())
	}
}
