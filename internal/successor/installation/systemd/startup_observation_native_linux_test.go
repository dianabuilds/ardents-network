//go:build startup_manager_native

package systemd

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

// This selects a real system manager with the fixed Endpoint/socket resources.
// Fresh typed facts do not attest a running invocation or successful Startup.
func TestInstallationNativeFreshStartupProperties(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root Endpoint test invocation required")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	observation, err := ObserveStartup(ctx)
	if err != nil {
		t.Fatal("fresh original manager property observation", err)
	}
	if observation.Manager == "" || observation.Version == "" {
		t.Fatal("native manager identity unavailable")
	}
	if !Matches(observation.Rounds[1].EndpointService, "MainPID", "u", uint32(os.Getpid())) {
		t.Fatal("native test does not belong to original Endpoint MainPID")
	}
	for _, round := range observation.Rounds {
		if len(round.EndpointUnit) == 0 || len(round.EndpointService) == 0 || len(round.ReaderUnit) == 0 || len(round.ReaderSocket) == 0 || len(round.PublisherUnit) == 0 || len(round.PublisherSocket) == 0 {
			t.Fatal("native fixed-object round incomplete")
		}
	}
	unit, service, err := Endpoint(ctx)
	if err != nil {
		t.Fatal("independent subprocess property observation", err)
	}
	for index, pair := range [][2]Properties{{observation.Rounds[1].EndpointUnit, unit}, {observation.Rounds[1].EndpointService, service}} {
		for _, key := range []string{"Id", "ActiveState", "FragmentPath", "InvocationID", "ExecStartEx", "MainPID", "User", "Group", "Type", "RestrictAddressFamilies", "CapabilityBoundingSet", "NoNewPrivileges", "ProtectSystem", "ReadWritePaths"} {
			fresh, found := pair[0][key]
			original, known := pair[1][key]
			if found != known || found && (fresh.Type != original.Type || !bytes.Equal(fresh.Data, original.Data)) {
				t.Fatal("fresh typed value differs from independent busctl", index, key, fresh.Type, string(fresh.Data), original.Type, string(original.Data))
			}
		}
	}
}
