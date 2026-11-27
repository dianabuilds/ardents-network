//go:build linux && endpoint_launch_native && endpoint_enrollment_migration_native

package endpointinstalled_test

import (
	"os"
	"testing"

	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
)

// TestNativeEndpointEnrollmentV5ToV6 starts an actual v5 program before it
// stages and starts the v6 successor using the same authenticated history.
func TestNativeEndpointEnrollmentV5ToV6(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("Endpoint enrollment migration requires a root driver")
	}
	predecessor, err := endpointfixture.ReadEnrollmentV5Artifacts(os.Getenv("ARDENTS_ENROLLMENT_V5_ARTIFACTS"))
	if err != nil {
		t.Fatal(err)
	}
	stand, grant := provisionLaunchWithPermissions(t, 1000, true, &predecessor)
	endpointfixture.VerifyNativeEndpointReplacement(t, stand, grant, nil)
}
