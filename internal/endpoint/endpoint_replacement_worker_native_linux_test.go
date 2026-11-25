//go:build linux && endpoint_launch_native && document_worker_installed

package endpoint_test

import (
	"os"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route/transport"
	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
)

// TestNativeEndpointReplacementWithPublisher joins an already admitted Worker
// owning an actual publication before staging the next signed Release.
func TestNativeEndpointReplacementWithPublisher(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("Endpoint replacement qualification requires a root driver")
	}
	f, _ := installedServiceNetwork(t, string(transport.ClosedCarrierTCP))
	stand := provisionPlatformLedLaunchFixtureWithSuccessor(t, 1000, f, true, nil)
	credential := &syscall.Credential{Uid: stand.UID, Gid: stand.GID, Groups: []uint32{}}
	endpointfixture.InitializeInstalledServiceInstance(t, stand.Program, credential, stand.Base, stand.NetworkID)
	endpointfixture.VerifyNativeEndpointReplacement(t, stand.LaunchStand, stand.grantPermissions, func(t *testing.T, original [3]string) string {
		return endpointfixture.PublishAndFindSnapshotWorker(t, stand.AdministrationSock, original[0], stand.UID)
	})
}
