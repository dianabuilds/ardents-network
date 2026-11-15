package enrollment

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestForwardingCandidateRequiresPrivateExecutableDelivery(t *testing.T) {
	for _, scenario := range []struct {
		name string
		mode os.FileMode
		want error
	}{{"private executable", 0700, nil}, {"public executable", 0755, ErrInventory}} {
		t.Run(scenario.name, func(t *testing.T) {
			root := t.TempDir()
			files := make(map[string][]byte)
			for _, name := range []string{"ardents-node-linux-amd64", "ardents-forwarding.service", "infrastructure-forwarding.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"} {
				files[name] = []byte("opaque public candidate input: " + name)
			}
			inventory, err := PrepareInfrastructureInventory(InfrastructureFacts{"linux-amd64", "candidate-access", 2, "forwarding"}, files)
			if err != nil {
				t.Fatal(err)
			}
			files[InfrastructureInventoryName] = inventory
			for name, raw := range files {
				mode := os.FileMode(0600)
				if name == "ardents-node-linux-amd64" {
					mode = scenario.mode
				}
				path := filepath.Join(root, name)
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, mode); err != nil {
					t.Fatal(err)
				}
				info, err := os.Lstat(path)
				if err != nil || info.Mode().Perm() != mode {
					t.Fatal("invalid environment: actual native access control differs", err)
				}
			}
			candidate, err := ReadInfrastructureCandidate(t.Context(), root)
			if !errors.Is(err, scenario.want) {
				t.Fatal("candidate access admission differs", err, scenario.want)
			}
			facts, accepted := candidate.Facts()
			if scenario.want != nil {
				_, exposed := candidate.File("ardents-node-linux-amd64")
				if accepted || exposed {
					t.Fatal("refused public delivery exposed a candidate")
				}
				return
			}
			if !accepted || facts.Invocation != "forwarding" || candidate.Check() != nil {
				t.Fatal("private executable delivery lacks a complete retained candidate")
			}
			for name, want := range files {
				got, found := candidate.File(name)
				if !found || !bytes.Equal(got, want) {
					t.Fatal("private candidate lost an original file", name)
				}
			}
		})
	}
}
