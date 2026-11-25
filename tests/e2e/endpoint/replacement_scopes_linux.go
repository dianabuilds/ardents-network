//go:build linux && endpoint_launch_native

package endpoint

import (
	"os"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/platform/cgroup"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// RequireStoppedEndpointScopes checks the original kernel scopes and activation sockets after Stop.
func RequireStoppedEndpointScopes(t testing.TB, scopes []*os.File) {
	t.Helper()
	for _, originalScope := range scopes {
		if populated, err := LaunchScopePopulated(originalScope); err != nil || populated {
			t.Fatalf("original scope did not join: populated=%v err=%v", populated, err)
		}
	}
	for _, role := range []string{"reader", "publisher"} {
		unit, _, err := systemd.Activation(t.Context(), role)
		if err != nil {
			t.Fatalf("observe stopped %s activation socket: %v", role, err)
		}
		if !systemd.Matches(unit, "Id", "s", "ardents-text-"+role+".socket") ||
			!systemd.Matches(unit, "ActiveState", "s", "inactive") ||
			!systemd.Matches(unit, "SubState", "s", "dead") ||
			!systemd.Matches(unit, "Job", "(uo)", []any{uint32(0), "/"}) {
			t.Fatalf("activation socket %s did not stop cleanly", role)
		}
	}
	if err := cgroup.ObserveEmpty(t.Context()); err != nil {
		t.Fatal("observe empty Endpoint and worker scope inventory:", err)
	}
}
