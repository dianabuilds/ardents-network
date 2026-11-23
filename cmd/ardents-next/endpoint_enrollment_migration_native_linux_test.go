//go:build linux && endpoint_launch_native && endpoint_enrollment_migration_native

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNativeEndpointEnrollmentV5ToV6 starts an actual v5 program before it
// stages and starts the v6 successor using the same authenticated history.
func TestNativeEndpointEnrollmentV5ToV6(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("Endpoint enrollment migration requires a root driver")
	}
	predecessor := os.Getenv("ARDENTS_ENROLLMENT_V5_ARTIFACTS")
	if !filepath.IsAbs(predecessor) {
		t.Fatal("provide an absolute directory with the pinned Linux amd64 v5 Endpoint and Custody artifacts")
	}
	stand := provisionPlatformLedLaunchFixtureWithSuccessor(t, 1000, newNetworkAdmissionFixture(t), true, predecessor)
	verifyNativeEndpointReplacement(t, stand, nil)
}
