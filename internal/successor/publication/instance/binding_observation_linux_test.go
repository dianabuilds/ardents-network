package instance

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication/durable"
)

// Genuine native custody/reservation observations; no qualified Publisher,
// accepting registration, JOIN or successful Connection is supplied here.
func TestBindingObservationRechecksOriginalNativeOwners(t *testing.T) {
	for _, mode := range []string{"original", "canceled", "history-floor", "instance-inode"} {
		t.Run(mode, func(t *testing.T) {
			root, config := acceptedNativeRoot(t)
			history, err := durable.Open(t.Context(), config)
			if err != nil {
				_ = root.Close()
				t.Fatal(err)
			}
			binding, err := root.Bind(t.Context(), history)
			if err != nil {
				_ = root.Close()
				_ = history.Close()
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = binding.Close(); _ = root.Close(); _ = history.Close() })
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch mode {
			case "canceled":
				cancel()
			case "history-floor":
				if err := os.WriteFile(filepath.Join(config.Root, "floor"), []byte("1\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "instance-inode":
				if err := os.Rename(root.path, root.path+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(root.path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := binding.observe(ctx); (err == nil) != (mode == "original") {
				t.Fatal("native observation accepted changed custody/history or refused original", mode, err)
			}
		})
	}
}

func TestBindingRetirementJoinsOriginalObservationBeforeErasure(t *testing.T) {
	root, config := acceptedNativeRoot(t)
	history, err := durable.Open(t.Context(), config)
	if err != nil {
		_ = root.Close()
		t.Fatal(err)
	}
	binding, err := root.Bind(t.Context(), history)
	if err != nil {
		_ = root.Close()
		_ = history.Close()
		t.Fatal(err)
	}
	lifetime, err := binding.BeginPublication(t.Context())
	if err != nil {
		_ = root.Close()
		_ = history.Close()
		t.Fatal(err)
	}
	if err := binding.Consume(t.Context()); err != nil {
		_ = lifetime.Close()
		_ = root.Close()
		_ = history.Close()
		t.Fatal(err)
	}
	// Retain one original physical observer at its actual admission barrier.
	// This controls only the join; it cannot produce an accepting observation.
	root.mu.Lock()
	binding.observations.Add(1)
	root.mu.Unlock()
	returned := false
	t.Cleanup(func() {
		if !returned {
			binding.observations.Done()
		}
		_ = lifetime.Close()
		_ = root.Close()
		_ = history.Close()
	})
	closed := make(chan error, 1)
	go func() { closed <- binding.Close() }()
	select {
	case <-lifetime.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("retirement did not revoke original Publication before join")
	}
	if err := lifetime.Close(); err != nil {
		t.Fatal(err)
	}
	root.mu.Lock()
	retained := len(binding.private) == 64
	denied := binding.checkLocal(t.Context()) != nil
	root.mu.Unlock()
	select {
	case err := <-closed:
		t.Error("binding returned before original observer joined", err)
	default:
	}
	if !retained || !denied {
		t.Error("retirement erased material early or admitted work during observer join")
	}
	binding.observations.Done()
	returned = true
	if err := binding.Close(); err != nil {
		t.Fatal(err)
	}
	root.mu.Lock()
	retained = len(binding.private) != 0
	root.mu.Unlock()
	if retained {
		t.Fatal("joined retirement retained private Instance material")
	}
}
