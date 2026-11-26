//go:build linux && network_signing_native

package provisioning

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func timedNetworkSigningArguments(t *testing.T, now, until time.Time) []string {
	t.Helper()
	parent := sourceArtifactNativeParent(t)
	p, key, _, _ := signingRecordFixture(t)
	p["valid_from"], p["valid_until"] = now.Add(-time.Minute).Format(time.RFC3339), until.Format(time.RFC3339)
	_, message, err := networkdraft.Prepare("prepare-initial-node-record", preparationJSON(t, p))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(message)
	plan, prepared, keyPath := filepath.Join(parent, "plan"), filepath.Join(parent, "prepared"), filepath.Join(parent, "key")
	if err = os.WriteFile(plan, signingDeclaration(t, "node-record", p, now), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(prepared, message, 0600); err != nil {
		t.Fatal(err)
	}
	encoded, err := artifactio.EncodeEd25519Key(key)
	clear(key)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(encoded)
	if err := os.WriteFile(keyPath, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return []string{"sign-initial-node-record", plan, prepared, preparationHex(digest[:]), keyPath, filepath.Join(parent, "signed")}
}

func TestNetworkSigningOriginalTimeAfterIO(t *testing.T) {
	for _, stage := range []string{"inputs-read", "before-directory-sync"} {
		t.Run(stage, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			until := now.Add(3 * time.Second)
			args := timedNetworkSigningArguments(t, now, until)
			observed := false
			_, err := SignNetwork(t.Context(), args, func(step string, _ *os.File) {
				if step != stage {
					return
				}
				observed = true
				// This waits for an actual clock boundary, not physical completion.
				// The original synchronous operation retains every input descriptor.
				if d := time.Until(until); d > 0 {
					time.Sleep(d + time.Millisecond)
				}
				if time.Now().Before(until) {
					t.Fatal("original clock prerequisite changed")
				}
			})
			if !observed || err == nil {
				t.Fatal("expired original time accepted after native IO", err)
			}
			_, statErr := os.Stat(args[5])
			if stage == "inputs-read" && !os.IsNotExist(statErr) {
				t.Fatal("expiry before signature created output", statErr)
			}
			if stage == "before-directory-sync" && statErr != nil {
				t.Fatal("uncertain output prefix removed", statErr)
			}
			healthy := time.Now().UTC().Truncate(time.Second)
			control := timedNetworkSigningArguments(t, healthy, healthy.Add(time.Hour))
			if _, e := SignNetwork(t.Context(), control, nil); e != nil {
				t.Fatal("independent healthy time control", e)
			}
		})
	}
}

func TestNetworkSigningDurableInventory(t *testing.T) {
	for _, fault := range []string{"healthy", "existing", "file replacement", "hardlink", "extra", "late bytes", "commit bytes", "directory replacement", "write close", "dir sync close", "cancel"} {
		t.Run(fault, func(t *testing.T) {
			parent := sourceArtifactNativeParent(t)
			root := filepath.Join(parent, "signed")
			if fault == "existing" {
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
			}
			artifacts := []artifactio.Content{{Name: "epoch.bin", Data: bytes.Repeat([]byte{3}, 512)}, {Name: "input-00.bin", Data: []byte("input")}, {Name: "material-00.bin", Data: []byte("material")}}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			acted := false
			err := PublishNetworkInventory(ctx, root, artifacts, false, func() error { return ctx.Err() }, func(step string, f *os.File) {
				if acted {
					return
				}
				var e error
				if step == "file-created" {
					switch fault {
					case "file replacement":
						e = os.Rename(filepath.Join(root, "epoch.bin"), filepath.Join(parent, "original"))
						if e == nil {
							e = os.WriteFile(filepath.Join(root, "epoch.bin"), artifacts[0].Data, 0600)
						}
						acted = true
					case "hardlink":
						e = os.Link(filepath.Join(root, "epoch.bin"), filepath.Join(parent, "link"))
						acted = true
					case "write close":
						e = f.Close()
						acted = true
					case "cancel":
						cancel()
						acted = true
					}
				}
				if step == "before-directory-sync" {
					switch fault {
					case "extra":
						e = os.WriteFile(filepath.Join(root, "foreign"), []byte("keep"), 0600)
						acted = true
					case "late bytes":
						e = os.WriteFile(filepath.Join(root, "epoch.bin"), []byte("changed"), 0600)
						acted = true
					case "directory replacement":
						e = os.Rename(root, root+"-original")
						if e == nil {
							e = os.Mkdir(root, 0700)
						}
						acted = true
					}
				}
				if step == "directory-sync" && fault == "dir sync close" {
					e = f.Close()
					acted = true
				}
				if step == "commit" && fault == "commit bytes" {
					e = os.WriteFile(filepath.Join(root, "epoch.bin"), []byte("changed after input reobservation"), 0600)
					acted = true
				}
				if e != nil {
					t.Fatal(e)
				}
			})
			if (err == nil) != (fault == "healthy") {
				t.Fatal("durability verdict", err)
			}
			if fault == "write close" && !errors.Is(err, os.ErrClosed) {
				t.Fatal("physical errors not retained", err)
			}
			if fault != "healthy" && fault != "existing" && !acted {
				t.Fatal("fault did not execute")
			}
			if e := PublishNetworkInventory(t.Context(), root, artifacts, false, func() error { return nil }, nil); e == nil {
				t.Fatal("prefix adopted")
			}
		})
	}
}

func TestNetworkSigningConcurrentOutput(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	root := filepath.Join(parent, "signed")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		wg.Go(func() {
			<-start
			results <- PublishNetworkInventory(t.Context(), root, []artifactio.Content{{Name: "node-record.bin", Data: []byte("signed")}}, false, func() error { return nil }, nil)
		})
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("exclusive output winners", success)
	}
}

func TestNetworkSigningUnsupportedMount(t *testing.T) {
	sourceArtifactNativeParent(t)
	parent, err := os.MkdirTemp("/dev/shm", "ardents-signing-mount-")
	if err != nil {
		t.Fatal("tmpfs refusal prerequisite", err)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(parent); e != nil {
			t.Error(e)
		}
	})
	root := filepath.Join(parent, "signed")
	if err = PublishNetworkInventory(t.Context(), root, []artifactio.Content{{Name: "node-record.bin", Data: []byte("signed")}}, false, func() error { return nil }, nil); err == nil {
		t.Fatal("unsupported mount accepted")
	}
	if _, err = os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("mount refusal created output", err)
	}
}
