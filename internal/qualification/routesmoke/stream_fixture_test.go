package routesmoke_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/routesmoke"
)

func TestStreamFixtureBindsOnlyEndpointLocalStreamSockets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "route")
	fixture, err := routesmoke.PrepareStreamFixture(root, "/run/client/route.sock", "/run/publisher/route.sock",
		time.Now().UTC().Truncate(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if fixture.NetworkID == [32]byte{} || fixture.ManifestDigest == [32]byte{} {
		t.Fatal("stream fixture identity is incomplete")
	}
	for role, socket := range map[string]string{"client": "/run/client/route.sock", "publisher": "/run/publisher/route.sock"} {
		raw, readErr := os.ReadFile(filepath.Join(root, "plans", role+".json"))
		if readErr != nil || !contains(raw, []byte(socket)) {
			t.Fatalf("%s plan does not bind its local stream: %v", role, readErr)
		}
	}
}

func contains(value, wanted []byte) bool {
	for index := 0; index+len(wanted) <= len(value); index++ {
		match := true
		for offset := range wanted {
			match = match && value[index+offset] == wanted[offset]
		}
		if match {
			return true
		}
	}
	return false
}
