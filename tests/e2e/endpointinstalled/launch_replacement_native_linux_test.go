//go:build linux && endpoint_launch_native

package endpointinstalled_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// This native qualification proves the FULL platform-led launch chain to
// protected text Service readiness for ONE dual-role (publisher+reader) Endpoint
// on the real admitted Ubuntu/systemd host (#391 S1). Platform preparation
// sets up the account and Worker resources, independently initializes trust,
// seals the generation via the production platform command and prepares the genuine
// histories, installs the platform main unit (sole privileged F-25 ExecStartPre),
// lets systemd start the participant (no root ACK), grants permissions through an
// independent fixture granter and observes the original-process listeners.
//
// It is component/acceptance evidence with fixture Release/Network authority for
// one dual-role Endpoint. It is NOT the two-participant privacy qualification
// (that is #391 S2, two separately admitted managers). The transitional
// guarded-start/ACK/recovery orchestration is not exercised or retired here.

func TestNativeEndpointPlatformLedLaunch(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("endpoint platform-led launch qualification requires a root driver")
	}
	if _, err := exec.Command("/usr/bin/busctl", "--system", "--no-pager", "call", "org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties", "Get", "ss", "org.freedesktop.systemd1.Manager", "Version").Output(); err != nil {
		t.Fatal("real platform systemd manager unavailable:", err)
	}
	stand, grant := provisionLaunchWithPermissions(t, 1000, true, nil)
	endpointfixture.VerifyNativeEndpointReplacement(t, stand, grant, nil)
}

func provisionLaunchWithPermissions(t *testing.T, refreshMilliseconds int, successor bool, predecessor *endpointfixture.EnrollmentV5Artifacts) (*endpointfixture.LaunchStand, func(*testing.T, [3]string)) {
	t.Helper()
	fixture := networkinputs.OpenSignedAdmission(t)
	spec := fixture.Spec
	view, err := fixture.State.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	facts, _, err := networkinputs.AdmissionFacts(view)
	if err != nil {
		t.Fatal(err)
	}
	stand := endpointfixture.ProvisionLaunchStand(t, buildArdentsNext(t), refreshMilliseconds, spec, successor, predecessor)
	grant := func(t *testing.T, original [3]string) {
		t.Helper()
		endpointfixture.GrantOriginalPermissions(t, stand.Base, original[1], stand.UID, stand.GID, 30*time.Second,
			func(_ string, role string, request []byte) []byte {
				return endpointfixture.ApproveFixturePermission(t, filepath.Join(stand.Base, "allocation-"+role), request, facts, spec.Authority, time.Now(), true)
			})
	}
	return stand, grant
}
