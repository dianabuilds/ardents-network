package durable

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// These tests exercise exact storage rules with a controlled flush mechanism.
// Successful native durability is separately exercised on Linux, never inferred
// from this portable ordering/failure fixture.
func TestFloorCanonicalAndRefusal(t *testing.T) {
	for _, raw := range []string{"1\n", "18446744073709551615\n", "18446744073709551616\n", "0\n", "01\n", "1", "+1\n", "1\r\n", "\n"} {
		t.Run(raw, func(t *testing.T) {
			config := Config{Root: filepath.Join(t.TempDir(), "publication"), Target: [32]byte{1}, Network: [32]byte{2}}
			flush := func(string) error { return nil }
			root, err := openWith(context.Background(), config, flush)
			if err != nil {
				t.Fatal(err)
			}
			if err = root.Close(); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(config.Root, "floor"), []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			root, err = openWith(context.Background(), config, flush)
			valid := raw == "1\n" || raw == "18446744073709551615\n"
			if !valid {
				if err == nil {
					root.Close()
					t.Fatal("malformed floor accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			floor, err := root.Floor(context.Background())
			if err != nil || (raw == "1\n" && floor != 1) || (raw != "1\n" && floor != ^uint64(0)) {
				t.Fatalf("floor %d: %v", floor, err)
			}
			if _, err = openWith(context.Background(), config, flush); err == nil {
				t.Fatal("exclusive lease reused")
			}
			kept, err := os.ReadFile(filepath.Join(config.Root, "floor"))
			if err != nil || string(kept) != raw {
				t.Fatal("floor changed", err)
			}
		})
	}
}

func TestMissingFloorAndForeignResidue(t *testing.T) {
	for _, name := range []string{"current", "retired-generation", "generations/0000000000000001"} {
		t.Run(name, func(t *testing.T) {
			config := Config{Root: filepath.Join(t.TempDir(), "publication"), Target: [32]byte{1}, Network: [32]byte{2}}
			flush := func(string) error { return nil }
			root, err := openWith(context.Background(), config, flush)
			if err != nil {
				t.Fatal(err)
			}
			root.Close()
			path := filepath.Join(config.Root, name)
			if name == "generations/0000000000000001" {
				err = os.Mkdir(path, 0700)
			} else {
				err = os.WriteFile(path, []byte("0000000000000001\n"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if root, err = openWith(context.Background(), config, flush); err == nil {
				root.Close()
				t.Fatal("unowned or floorless history accepted")
			}
			if _, err = os.Lstat(path); err != nil {
				t.Fatal("refusal changed history", err)
			}
		})
	}
}

func TestRequiredLayoutFlushAndClose(t *testing.T) {
	for boundary := 1; boundary <= 5; boundary++ {
		config := Config{Root: filepath.Join(t.TempDir(), "parent", "publication"), Target: [32]byte{1}, Network: [32]byte{2}}
		fault := errors.New("required directory flush failed")
		calls := 0
		root, err := openWith(context.Background(), config, func(string) error {
			calls++
			if calls == boundary {
				return fault
			}
			return nil
		})
		if !errors.Is(err, fault) || root != nil {
			if root != nil {
				root.Close()
			}
			t.Fatalf("boundary %d accepted: %v", boundary, err)
		}
	}
	config := Config{Root: filepath.Join(t.TempDir(), "publication"), Target: [32]byte{1}, Network: [32]byte{2}}
	root, err := openWith(context.Background(), config, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := root.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if _, err = root.Floor(context.Background()); err == nil {
		t.Fatal("closed root supplied floor")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = openWith(cancelled, config, func(string) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
