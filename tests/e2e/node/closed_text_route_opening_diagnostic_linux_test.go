//go:build linux && text_worker_installed

package state_test

import (
	"os"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// This diagnostic retains the installed command and cleanup boundaries but
// stops before the full profile's timed Descriptor refresh observation.
func TestInstalledClosedTextRouteOpeningDiagnostic(t *testing.T) {
	if os.Geteuid() != 0 || os.Getenv("ARDENTS_TEXT_COMMAND_QUALIFICATION") != "1" || os.Getenv("ARDENTS_E2E_COMMAND_ROOT") == "" {
		t.Fatal("invalid environment: select the dedicated installed root command profile with prebuilt commands")
	}
	authority := createClosedCommandAuthority(t, [32]byte{1})
	testClosedIssuerProvisioningParticipant(t, "ardents-carrier-quic-v2", 16, authority.Public, nil, func(config state.Config, binary, resolutionRoot string, sourcePlan map[string]any) {
		runInstalledClosedTextParticipant(t, config, binary, resolutionRoot, authority, sourcePlan, nil, false)
	})
}
