//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
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
	err = executeEndpointPlatformCheck(t.Context(), []string{"check", "/etc/ardents/endpoint.json", strings.Repeat("0", 64)}, &out, &artifacts.PlatformInputs{}, raw)
	if err == nil || out.Len() != 0 {
		t.Fatalf("wrong pin reached effects: err=%v output=%q", err, out.String())
	}
}

func endpointLaunchFixture() platformLaunch {
	return platformLaunch{
		Role:          "endpoint",
		Distribution:  "/var/lib/ardents/artifacts/release-one",
		ReleaseRoot:   "/var/lib/ardents/release-trust",
		Environment:   "closed",
		Network:       "private",
		Program:       "/var/lib/ardents/endpoint/generations/" + strings.Repeat("a", 64) + "/ardents-linux-amd64",
		Configuration: "/etc/ardents/endpoint.json",
		ConfigSHA256:  strings.Repeat("a", 64),
		WritableRoots: []string{"/var/lib/ardents/roles/endpoint/state", "/var/lib/ardents/roles/endpoint/entry"},
		Inputs: map[string]string{
			"/etc/ardents/client.crt": strings.Repeat("c", 64),
			"/etc/ardents/client.key": strings.Repeat("d", 64),
		},
	}
}
