//go:build linux

package enrollment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/domain/enrollment/infrastructureinventory"
	"testing"
)

// Inventory hashes alone do not admit a delivery with public native access.
// This reader control creates no Release proof or executable Node authority.
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
			inventory, err := infrastructureinventory.Prepare(infrastructureinventory.Facts{
				Platform: "linux-amd64", ReleaseIdentity: "candidate-access",
				ReleaseVersion: 2, Invocation: "forwarding",
			}, files)
			if err != nil {
				t.Fatal(err)
			}
			files[infrastructureinventory.Name] = inventory
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

func TestInfrastructurePinPrecedesInventoryParsing(t *testing.T) {
	root := t.TempDir()
	raw := []byte("malformed inventory\n")
	if err := os.WriteFile(filepath.Join(root, infrastructureinventory.Name), raw, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	for _, scenario := range []struct {
		name string
		pin  string
		want error
	}{
		{"wrong pin", strings.Repeat("0", 64), ErrPin},
		{"exact pin", hex.EncodeToString(digest[:]), ErrInventory},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			bundle, err := VerifyInfrastructure(t.Context(), root, scenario.pin)
			_, facts := bundle.Facts()
			_, file := bundle.File(infrastructureinventory.Name)
			if !errors.Is(err, scenario.want) || facts || file || bundle.Check() == nil {
				t.Fatalf("refusal returned facts=%t file=%t error=%v, want %v", facts, file, err, scenario.want)
			}
		})
	}
}
