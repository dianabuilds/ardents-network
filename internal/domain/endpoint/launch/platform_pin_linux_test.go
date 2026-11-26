//go:build linux

package launch

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/plan"
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
	result, err := VerifyPlatform(t.Context(), &artifacts.PlatformInputs{}, PlatformCheckStart, "/etc/ardents/endpoint.json", digestHex(append([]byte("foreign"), raw...)), raw)
	if err == nil || len(result) != 0 {
		t.Fatalf("wrong pin reached effects: err=%v output=%q", err, result)
	}
}

func endpointLaunchFixture() plan.PlatformLaunch {
	return plan.PlatformLaunch{
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
