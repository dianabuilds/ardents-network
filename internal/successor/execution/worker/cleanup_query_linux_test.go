//go:build linux

package worker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestCleanupCollectedUnitQueryCannotReloadItsName(t *testing.T) {
	instance, unit, service := cleanupObservation(t)
	// A collected systemd instance can be loaded again by its name with no
	// invocation. The actual native reproduction returned this empty ay,
	// while Id, LoadState and the fixed protection properties still matched.
	unit["InvocationID"] = value{Type: "ay", Data: json.RawMessage(`[]`)}
	const originalPath = "/org/freedesktop/systemd1/unit/01000000000000000000000000000000"
	namePath := "/org/freedesktop/systemd1/unit/retired_name"
	query := func(_ context.Context, path, _, method, _ string, arguments ...string) (value, error) {
		if path == originalPath {
			return value{}, errors.New("original invocation is no longer known")
		}
		if method == "GetUnit" {
			body, err := json.Marshal([]string{namePath})
			return value{Type: "o", Data: body}, err
		}
		if path != namePath || method != "GetAll" || len(arguments) != 1 {
			t.Fatal("unexpected manager query")
		}
		observed := unit
		if arguments[0] == "org.freedesktop.systemd1.Service" {
			observed = service
		}
		body, err := json.Marshal([]properties{observed})
		return value{Type: "a{sv}", Data: body}, err
	}
	owner := &Cleanup{instance: instance}
	reads := 0
	err := owner.joinObserved(context.Background(),
		func(*os.File) (bool, bool, error) {
			reads++
			// Independent original-event observations: initially populated,
			// then removed. A manager lookup alone never grants this result.
			return reads > 1, reads == 1, nil
		},
		func(ctx context.Context, _, _ string) (properties, properties, error) {
			return readCleanupProperties(ctx, instance, query)
		},
		func(context.Context, string, string) error {
			t.Fatal("collected original invocation authorized Stop by name")
			return nil
		})
	if err != nil {
		t.Fatalf("removed original cgroup was confused with a reloaded name: %v", err)
	}
	if reads < 2 {
		t.Fatal("unavailable original invocation bypassed original physical observation")
	}
}
