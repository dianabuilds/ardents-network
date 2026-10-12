package duty

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// These actual journal tests exercise local producer lifetime only. Supplied
// restriction facts do not establish authenticated State or Route admission.
func TestNodeProducerOriginalCallerAndForeignSourceRetention(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	config := Config{Root: filepath.Join(t.TempDir(), "roles"), Clock: func() time.Time { return now }, Create: true}
	store, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	source := [32]byte{7}
	sourceIdentity, sourceFamily := [32]byte{8}, [32]byte{9}
	if err := store.Replace(source, []Duty{{Identity: sourceIdentity, Family: sourceFamily, Class: "direct-source", State: "live", NotAfter: now.Add(time.Second)}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	config.Create = false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	facts := NodeParticipation{Identity: [32]byte{1}, Family: "node-family", Assignment: "initiator", EpochUntil: now.Add(time.Minute), RecordUntil: now.Add(2 * time.Minute)}
	producer, err := OpenNodeProducer(ctx, config, facts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = producer.Close(context.Background()) })
	if err := producer.Live(); err != nil {
		t.Fatal(err)
	}
	if err := producer.Check(); err != nil {
		t.Fatal(err)
	}
	// The producer does not hold a lease across its original runtime.
	store, err = OpenOperation(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	family := sha256.Sum256([]byte(facts.Family))
	if conflict, err := store.Conflict(facts.Identity, family); err != nil || !conflict {
		t.Fatalf("missing joined-use exclusion: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	copy := *producer
	cancel()
	if err := copy.Check(); err == nil {
		t.Fatal("canceled producer admitted work")
	}
	if err := copy.Live(); err == nil {
		t.Fatal("canceled producer recreated live record")
	}
	if err := copy.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := producer.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	store, err = Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if conflict, err := store.Conflict(facts.Identity, family); err != nil || conflict {
		t.Fatalf("joined producer retained exclusion: %v", err)
	}
	now = now.Add(3 * time.Minute)
	if conflict, err := store.Conflict(sourceIdentity, sourceFamily); err != nil || !conflict {
		t.Fatalf("Node erased or expired foreign live Source: %v", err)
	}
}

func TestNodeProducerReopenCannotRollbackOrForkObservedGeneration(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	config := Config{Root: filepath.Join(t.TempDir(), "roles"), Clock: func() time.Time { return now }, Create: true}
	facts := NodeParticipation{Identity: [32]byte{1}, FamilyID: [32]byte{2}, Assignment: "initiator", EpochUntil: now.Add(time.Minute), RecordUntil: now.Add(2 * time.Minute)}
	producer, err := OpenNodeProducer(context.Background(), config, facts)
	if err != nil {
		t.Fatal(err)
	}
	if err := producer.Live(); err != nil {
		t.Fatal(err)
	}
	// Capture a fully coherent prior root, not malformed or partially synced
	// bytes. Later restore only this test-owned root after all leases close.
	entries, err := os.ReadDir(config.Root)
	if err != nil {
		t.Fatal(err)
	}
	prior := make(map[string][]byte)
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(config.Root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		prior[entry.Name()] = raw
	}
	config.Create = false
	store, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Replace([32]byte{9}, []Duty{{Identity: [32]byte{10}, Family: [32]byte{11}, Class: "direct-source", State: "live", NotAfter: now.Add(time.Minute)}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := producer.Check(); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(config.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.Remove(filepath.Join(config.Root, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	for name, raw := range prior {
		if err := os.WriteFile(filepath.Join(config.Root, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := producer.Check(); err == nil {
		t.Fatal("coherent older history revived original producer")
	}
	store, err = Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Replace([32]byte{12}, []Duty{{Identity: [32]byte{13}, Family: [32]byte{14}, Class: "direct-source", State: "live", NotAfter: now.Add(time.Minute)}}); err != nil {
		t.Fatal(err)
	}
	if store.state.Generation != producer.original.generation || store.current == producer.original.current {
		t.Fatal("negative control did not create a coherent same-generation fork")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := producer.Check(); err == nil {
		t.Fatal("coherent same-generation fork revived original producer")
	}
	if err := producer.Live(); err == nil {
		t.Fatal("rolled-back history admitted live replacement")
	}
	if err := producer.Close(context.Background()); err == nil {
		t.Fatal("rolled-back history reported successful removal")
	}
	store, err = Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if len(store.state.Duties) != 2 {
		t.Fatal("failed original cleanup changed restored history")
	}
}

func TestNodeProducerExpiryAndFailedCleanupRetainHistory(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	config := Config{Root: filepath.Join(t.TempDir(), "roles"), Clock: func() time.Time { return now }, Create: true}
	facts := NodeParticipation{Identity: [32]byte{1}, FamilyID: [32]byte{2}, Assignment: "initiator", EpochUntil: now.Add(time.Minute), RecordUntil: now.Add(2 * time.Minute)}
	producer, err := OpenNodeProducer(context.Background(), config, facts)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate, err := OpenNodeProducer(context.Background(), config, facts); !errors.Is(err, ErrLocalRoleConflict) || duplicate != nil {
		t.Fatalf("duplicate original admitted: %v", err)
	}
	cleanup, cancel := context.WithCancel(context.Background())
	cancel()
	first := producer.Close(cleanup)
	if !errors.Is(first, context.Canceled) || producer.Close(context.Background()) != first {
		t.Fatal("failed cleanup became successful")
	}
	config.Create = false
	store, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if conflict, err := store.Conflict(facts.Identity, facts.FamilyID); err != nil || !conflict {
		t.Fatal("failed cleanup erased conflict", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	now = facts.EpochUntil
	if err := producer.Check(); err == nil {
		t.Fatal("expired/closed producer admitted work")
	}
	store, err = Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if len(store.state.Duties) != 1 {
		t.Fatal("expiration erased retained journal without transaction")
	}
}

func TestNodeProducerConcurrentCopiesCloseOnce(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	config := Config{Root: filepath.Join(t.TempDir(), "roles"), Clock: func() time.Time { return now }, Create: true}
	facts := NodeParticipation{Identity: [32]byte{1}, FamilyID: [32]byte{2}, Assignment: "initiator", EpochUntil: now.Add(time.Minute), RecordUntil: now.Add(2 * time.Minute)}
	producer, err := OpenNodeProducer(context.Background(), config, facts)
	if err != nil {
		t.Fatal(err)
	}
	copy := *producer
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			if err := copy.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	if producer.Check() == nil || producer.Live() == nil {
		t.Fatal("retired copy revived original")
	}
	config.Create = false
	store, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if len(store.state.Duties) != 0 {
		t.Fatal("joined original retained conflict")
	}
}

func TestNodeProducerExpiryDuringOriginalJournalReadRefusesHandoff(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	end := now.Add(time.Minute)
	armed, reads := false, 0
	config := Config{Root: filepath.Join(t.TempDir(), "roles"), Create: true, Clock: func() time.Time {
		if armed {
			reads++
			if reads == 2 {
				now = end
			}
		}
		return now
	}}
	facts := NodeParticipation{Identity: [32]byte{1}, FamilyID: [32]byte{2}, Assignment: "initiator", EpochUntil: end, RecordUntil: end.Add(time.Minute)}
	producer, err := OpenNodeProducer(context.Background(), config, facts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = producer.Close(context.Background()) })
	if err := producer.Live(); err != nil {
		t.Fatal(err)
	}
	// Admission observes currentness first; the original durable Open then
	// observes expiration. The same unchanged fact must not become a permit
	// merely because it was current before journal I/O.
	armed = true
	if err := producer.Check(); err == nil || reads < 3 {
		t.Fatal("post-I/O expiry published successful original check", err)
	}
	armed = false
	config.Create = false
	store, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if len(store.state.Duties) != 1 {
		t.Fatal("refusal erased original conflict without join")
	}
}

func TestNodeProducerMissingHistoryCannotRecreateOriginal(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	config := Config{Root: filepath.Join(t.TempDir(), "roles"), Clock: func() time.Time { return now }, Create: true}
	facts := NodeParticipation{Identity: [32]byte{1}, Family: "node-family", Assignment: "initiator", EpochUntil: now.Add(time.Minute), RecordUntil: now.Add(2 * time.Minute)}
	producer, err := OpenNodeProducer(context.Background(), config, facts)
	if err != nil {
		t.Fatal(err)
	}
	config.Create = false
	store, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Remove(producer.original.producer); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := producer.Check(); err == nil {
		t.Fatal("removed original passed Check")
	}
	if err := producer.Live(); err == nil {
		t.Fatal("removed original recreated participation")
	}
	first := producer.Close(context.Background())
	if first == nil || producer.Close(context.Background()) != first {
		t.Fatal("cleanup lost original refusal")
	}
	store, err = Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if len(store.state.Duties) != 0 {
		t.Fatal("refusal recreated removed history")
	}
}
