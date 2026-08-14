package routesmoke_test

import (
	"fmt"
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

func TestRecoveryStreamFixtureCreatesThreeDistinctCandidatesPerRole(t *testing.T) {
	root := filepath.Join(t.TempDir(), "route")
	fixture, err := routesmoke.PrepareRecoveryStreamFixture(root, "/run/client/route.sock",
		"/run/publisher/route.sock", time.Now().UTC().Truncate(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	identities := map[[32]byte]bool{}
	endpoints := map[string]bool{}
	for _, candidate := range fixture.Candidates {
		counts[candidate.Role]++
		if candidate.NodeID == [32]byte{} || candidate.PublicKey == [32]byte{} || identities[candidate.NodeID] ||
			endpoints[candidate.Endpoint] {
			t.Fatalf("recovery candidate is not distinct: %+v", candidate)
		}
		identities[candidate.NodeID], endpoints[candidate.Endpoint] = true, true
	}
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder"} {
		if counts[role] != 3 {
			t.Fatalf("role %s candidates=%d; want 3", role, counts[role])
		}
		for suffix := 1; suffix <= 3; suffix++ {
			name := role
			if suffix > 1 {
				name += "-" + string(rune('0'+suffix))
			}
			if _, err := os.Stat(filepath.Join(root, "plans", name+".json")); err != nil {
				t.Fatalf("role %s plan: %v", name, err)
			}
		}
	}
	for index, endpoint := range fixture.RouteCase.Endpoints {
		want := fmt.Sprintf("172.31.20.%d:%d", 11+index, 4601+index)
		if endpoint != want {
			t.Fatalf("initial %s endpoint=%s; want %s", []string{"initiator", "introduction", "rendezvous", "responder"}[index], endpoint, want)
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
