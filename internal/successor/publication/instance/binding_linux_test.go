package instance

import (
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication/durable"
)

func acceptedNativeRoot(t *testing.T) (*Root, durable.Config) {
	t.Helper()
	config := nativeConfig(t)
	root, err := Initialize(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = root.Accept(context.Background(), fixtureResponse(root.state.request, 7, 3)); err != nil {
		root.Close()
		t.Fatal(err)
	}
	credential, err := root.Credential(context.Background())
	if err != nil {
		root.Close()
		t.Fatal(err)
	}
	value := credential.Delegation()
	return root, durable.Config{Root: filepath.Join(t.TempDir(), "publication"), Target: value.Target, Network: value.Network}
}

func TestNativeBindingConsumesActualFloorAndRedactsRestart(t *testing.T) {
	root, config := acceptedNativeRoot(t)
	history, err := durable.Open(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	binding, err := root.Bind(context.Background(), history)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := any(binding).(crypto.Signer); ok {
		t.Fatal("binding exposes generic signing")
	}
	if _, err = root.Bind(context.Background(), history); err == nil {
		t.Fatal("second binding acquired original generation")
	}
	if err = binding.Consume(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = binding.Consume(context.Background()); err != nil {
		t.Fatal("exact consumption retry", err)
	}
	if floor, err := history.Floor(context.Background()); err != nil || floor != 7 {
		t.Fatal(floor, err)
	}
	raw, err := os.ReadFile(filepath.Join(config.Root, "floor"))
	if err != nil || string(raw) != "7\n" {
		t.Fatal("canonical floor absent", err)
	}
	privateState, err := os.ReadFile(filepath.Join(root.path, stateName))
	if err != nil {
		t.Fatal(err)
	}
	var disk storedState
	if json.Unmarshal(privateState, &disk) != nil || disk.Phase != Consumed || disk.Private != "" {
		t.Fatal("persisted private material survived consumption")
	}
	if len(binding.private) != 64 || len(root.state.private) != 0 {
		t.Fatal("live key is not confined to original binding")
	}
	path := root.path
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	if len(binding.private) != 0 || binding.Consume(context.Background()) == nil {
		t.Fatal("closed binding revived")
	}
	root, err = Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err = root.Credential(context.Background()); !errors.Is(err, ErrSuccessorRequired) {
		t.Fatal("consumed restart revived Credential", err)
	}
}

func TestNativeBindingReconcilesSpentAndForeignHistory(t *testing.T) {
	for _, mode := range []string{"spent", "foreign-target", "foreign-network"} {
		t.Run(mode, func(t *testing.T) {
			root, config := acceptedNativeRoot(t)
			defer root.Close()
			if mode == "foreign-target" {
				config.Target[0] ^= 1
			} else if mode == "foreign-network" {
				config.Network[0] ^= 1
			}
			history, err := durable.Open(context.Background(), config)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "spent" {
				history.Close()
				// Canonical history input represents a previous failed publication;
				// no successful live Publisher is supplied by this fixture.
				if err = os.WriteFile(filepath.Join(config.Root, "floor"), []byte("7\n"), 0600); err != nil {
					t.Fatal(err)
				}
				history, err = durable.Open(context.Background(), config)
				if err != nil {
					t.Fatal(err)
				}
			}
			defer history.Close()
			if binding, err := root.Bind(context.Background(), history); err == nil {
				binding.Close()
				t.Fatal("unreconciled history acquired binding")
			} else if mode == "spent" && !errors.Is(err, ErrSuccessorRequired) {
				t.Fatal(err)
			}
			if mode == "spent" && (root.state.phase != Consumed || len(root.state.private) != 0) {
				t.Fatal("spent accepted root retained key")
			}
		})
	}
}

func TestNativeHistoryCloseDeniesAndJoinsOriginalBinding(t *testing.T) {
	root, config := acceptedNativeRoot(t)
	history, err := durable.Open(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := root.Bind(context.Background(), history)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		group.Go(func() { results <- history.Close() })
	}
	// Wait for synchronous denial, while the exclusive lease remains retained.
	deadline := time.Now().Add(time.Second)
	for {
		if _, err = history.Floor(context.Background()); err != nil {
			break
		}
		if !time.Now().Before(deadline) {
			t.Fatal("history Close did not deny acquisitions")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case <-results:
		t.Fatal("root released before original binding joined")
	default:
	}
	if err = binding.Consume(context.Background()); err == nil {
		t.Fatal("late consumption committed after root denial")
	}
	if other, err := durable.Open(context.Background(), config); err == nil {
		other.Close()
		t.Fatal("unjoined lease allowed a second writer")
	}
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	group.Wait()
	for range 2 {
		if err = <-results; err != nil {
			t.Fatal(err)
		}
	}
	other, err := durable.Open(context.Background(), config)
	if err != nil {
		t.Fatal("joined lease did not reopen", err)
	}
	other.Close()
}

func TestNativeBindingConcurrentConsumptionRetainsOneOriginalFloor(t *testing.T) {
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
	results := make(chan error, 8)
	for range 8 {
		go func() { results <- binding.Consume(t.Context()) }()
	}
	for range 8 {
		if err := <-results; err != nil {
			t.Error("original concurrent consumption", err)
		}
	}
	if floor, err := history.Floor(t.Context()); err != nil || floor != 7 {
		t.Fatal("consumption changed original durable floor", floor, err)
	}
	root.mu.Lock()
	consumed := binding.consumed && len(binding.private) == 64 && root.state.phase == Consumed && len(root.state.private) == 0
	root.mu.Unlock()
	if !consumed {
		t.Fatal("consumption did not retain exact original private/persisted state")
	}
}

func TestNativeConcurrentBindingAcquiresOnlyOneOriginalReservation(t *testing.T) {
	root, config := acceptedNativeRoot(t)
	history, err := durable.Open(t.Context(), config)
	if err != nil {
		_ = root.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close(); _ = history.Close() })
	type result struct {
		binding *Binding
		err     error
	}
	results := make(chan result, 8)
	for range 8 {
		go func() { binding, err := root.Bind(t.Context(), history); results <- result{binding, err} }()
	}
	var original *Binding
	for range 8 {
		value := <-results
		if value.err == nil {
			if original != nil || value.binding == nil {
				t.Error("concurrent opening transferred another reservation")
			}
			original = value.binding
		} else if value.binding != nil || !errors.Is(value.err, ErrUnavailable) {
			t.Error("concurrent opening returned foreign state", value.binding, value.err)
		}
	}
	if original == nil || original.observe(t.Context()) != nil {
		t.Fatal("concurrent acquisition lost original native reservation")
	}
}

func TestNativeForeignBindingCannotEraseOriginalInstance(t *testing.T) {
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
	t.Cleanup(func() { _ = lifetime.Close(); _ = binding.Close(); _ = root.Close(); _ = history.Close() })
	if err := binding.Consume(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Deliberately share the original private backing bytes and lifetime, but
	// never copy a mutex or wait group. Public Binding possession is not identity.
	foreign := &Binding{root: root, generation: binding.generation, credential: binding.credential, private: binding.private, lifetime: lifetime}
	originalPrivate := append([]byte(nil), binding.private...)
	defer clear(originalPrivate)
	closed := make(chan error, 1)
	go func() { closed <- foreign.Close() }()
	select {
	case err := <-closed:
		if !errors.Is(err, ErrUnavailable) {
			t.Error("foreign binding acquired original retirement", err)
		}
	case <-lifetime.Context().Done():
		_ = lifetime.Close()
		<-closed
		t.Fatal("foreign binding canceled the original Publication")
	case <-time.After(time.Second):
		_ = lifetime.Close()
		<-closed
		t.Fatal("foreign retirement did not refuse promptly")
	}
	root.mu.Lock()
	unchanged := root.binding == binding && !binding.closed && bytes.Equal(binding.private, originalPrivate)
	root.mu.Unlock()
	if !unchanged || lifetime.Context().Err() != nil || binding.observe(t.Context()) != nil {
		t.Fatal("foreign retirement changed original Instance/Publication authority")
	}
}
