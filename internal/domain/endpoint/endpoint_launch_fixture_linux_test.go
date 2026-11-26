//go:build linux && endpoint_launch_native

package endpoint_test

import (
	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
	"path/filepath"
	"testing"
	"time"
)

type platformLedLaunchStand struct {
	*endpointfixture.LaunchStand
	f *networkAdmissionFixture
}

// provisionPlatformLedLaunchFixture provisions the same platform-led launch stand
// against an already-created Network admission fixture. The cold/warm latency
// harness (#650) supplies the publication Route network fixture so the installed
// participant reads through genuine Forwarding/Descriptor/Introduction/JOIN/
// Responder/Issuer receivers; the launch and NET-32 qualifications supply the
// ordinary idle fixture.
func provisionPlatformLedLaunchFixture(t *testing.T, refreshMS int, f *networkAdmissionFixture) *platformLedLaunchStand {
	t.Helper()
	return provisionPlatformLedLaunchFixtureWithSuccessor(t, refreshMS, f, false, nil)
}

func provisionPlatformLedLaunchFixtureWithSuccessor(t *testing.T, refreshMS int, f *networkAdmissionFixture, successor bool, predecessor *endpointfixture.EnrollmentV5Artifacts) *platformLedLaunchStand {
	t.Helper()
	stand := endpointfixture.ProvisionLaunchStand(t, commandBuild.Binary(t), refreshMS, f.spec, successor, predecessor)
	return &platformLedLaunchStand{LaunchStand: stand, f: f}
}

// platformLedDualDeclaration converts the reader fixture request to the dual
// publisher+reader role, preserving canonical field order and the newline.
func (s *platformLedLaunchStand) grantPermissions(t *testing.T, original [3]string) {
	t.Helper()
	endpointfixture.GrantOriginalPermissions(t, s.Base, original[1], s.UID, s.GID, 30*time.Second,
		func(_ string, role string, request []byte) []byte { return s.approve(t, role, request) })
}

func (s *platformLedLaunchStand) approve(t *testing.T, role string, request []byte) []byte {
	t.Helper()
	return endpointfixture.ApproveFixturePermission(t, filepath.Join(s.Base, "allocation-"+role), request, s.f.profile, s.f.spec.Authority, time.Now(), true)
}
