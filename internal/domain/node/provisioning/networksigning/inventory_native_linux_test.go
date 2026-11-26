//go:build linux && network_signing_native

package networksigning

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	provisionfixture "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func TestNetworkSigningDurableInventory(t *testing.T) {
	for _, fault := range []string{"healthy", "existing", "file replacement", "hardlink", "extra", "late bytes", "commit bytes", "directory replacement", "write close", "dir sync close", "cancel"} {
		t.Run(fault, func(t *testing.T) {
			parent := provisionfixture.ProtectedExt4Parent(t)
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
			err := publishInventory(ctx, root, artifacts, false, func() error { return ctx.Err() }, func(step string, f *os.File) {
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
			if e := publishInventory(t.Context(), root, artifacts, false, func() error { return nil }, nil); e == nil {
				t.Fatal("prefix adopted")
			}
		})
	}
}

func TestNetworkSigningConcurrentOutput(t *testing.T) {
	parent := provisionfixture.ProtectedExt4Parent(t)
	root := filepath.Join(parent, "signed")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		wg.Go(func() {
			<-start
			results <- publishInventory(t.Context(), root, []artifactio.Content{{Name: "node-record.bin", Data: []byte("signed")}}, false, func() error { return nil }, nil)
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
	provisionfixture.ProtectedExt4Parent(t)
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
	if err = publishInventory(t.Context(), root, []artifactio.Content{{Name: "node-record.bin", Data: []byte("signed")}}, false, func() error { return nil }, nil); err == nil {
		t.Fatal("unsupported mount accepted")
	}
	if _, err = os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("mount refusal created output", err)
	}
}
