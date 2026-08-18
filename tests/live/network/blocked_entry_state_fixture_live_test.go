//go:build live

package network_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBlockedFixturesCreateRestartPersistentStateRoots(t *testing.T) {
	fixture := newBlockedEntryFixture(t, "client", "server")
	assertBlockedStateRoots(t, fixture.root, "endpoint", "bridge")

	negative := newBlockedNegativeFixture(t, "client", "server")
	assertBlockedStateRoots(t, negative.root, "negative-endpoint", "recovery-endpoint", "fault-one")
}

func assertBlockedStateRoots(t *testing.T, root string, roles ...string) {
	t.Helper()
	for _, role := range roles {
		path := filepath.Join(root, "state", role)
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Fatalf("persistent state root %s: %v", role, err)
		}
	}
}
