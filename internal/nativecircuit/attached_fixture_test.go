package nativecircuit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAttachedFixtureKeepsRouteKnowledgeOpaque(t *testing.T) {
	fixture, err := prepareNativeFixtureMode(t.TempDir(), "gatec-attached", "", nil, &attachedSpec{
		userSocket: "/owned/user/app.sock", serviceSocket: "/owned/service/app.sock",
	})
	if err != nil {
		t.Fatal(err)
	}
	user, err := readRoleConfig(filepath.Join(fixture.root, "configs", "user", "role.json"))
	if err != nil {
		t.Fatal(err)
	}
	service, err := readRoleConfig(filepath.Join(fixture.root, "configs", "service", "role.json"))
	if err != nil {
		t.Fatal(err)
	}
	if user.AttachedSocket != "/attached/user/app.sock" || service.AttachedSocket != "/attached/service/app.sock" {
		t.Fatalf("attached sockets not role-scoped: user=%q service=%q", user.AttachedSocket, service.AttachedSocket)
	}
	if user.PayloadSeed != "" || user.PayloadBytes != 0 || user.StreamDirection != "" || service.StreamDirection != "" {
		t.Fatal("attached Route retained a synthetic workload")
	}
	data, err := os.ReadFile(filepath.Join(fixture.root, "compose-attached.yaml"))
	if err != nil || len(data) == 0 {
		t.Fatal("attached Compose override was not produced")
	}
	for _, role := range nativeNodeRoles {
		config, err := readRoleConfig(filepath.Join(fixture.root, "configs", role, "role.json"))
		if err != nil {
			t.Fatal(err)
		}
		if config.AttachedSocket != "" {
			t.Fatalf("Node %s learned an Application socket", role)
		}
	}
}
