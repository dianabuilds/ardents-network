package duty_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
)

// GAP-6 regression tests for the source-exposure ledger consulted at Entry
// admission.
//
// The original contract placed these tests beside a real `entry.Verify`
// call; the package-map rule (enforced by `internal/architecture`) forbids
// `internal/entry` and `internal/network/duty` from importing each other, so
// the tests live here, where a real `duty.Store` is constructible, and cover
// the duty side of the production chain: a `direct-source` duty retained by
// `Replace` is detected by `store.Conflict` and by `ReadConflict`.
//
// ADR-0106 retired the Invite subsystem, deleting the former `entry import`
// consumer that wired `validateInvite` into this ledger (and with it the
// `internal/entry/entry_test.go` coverage the earlier revision of this
// comment cited). The ledger itself stays live production code: its current
// consumer is the endpoint's closed Entry set selection, and these tests
// keep the four GAP-6 store-side behaviours covered.

func openStoreWithClock(t *testing.T) (localDutyStore, string, func() time.Time) {
	t.Helper()
	now := time.Unix(1_750_000_000, 0).UTC()
	root := filepath.Join(t.TempDir(), "local-roles")
	clock := func() time.Time { return now }
	store, err := localroles.Open(localroles.Config{Root: root, Clock: clock, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	return localDutyStore{store: store}, root, clock
}

// localDutyStore wraps the unexported *store so the external test package
// can call the four methods we need without naming the type directly.
type localDutyStore struct {
	store interface {
		Replace([32]byte, []localroles.Duty) error
		Conflict([32]byte, [32]byte) (bool, error)
		Close() error
	}
}

func (l localDutyStore) Replace(producer [32]byte, duties []localroles.Duty) error {
	return l.store.Replace(producer, duties)
}

func (l localDutyStore) Conflict(identity, family [32]byte) (bool, error) {
	return l.store.Conflict(identity, family)
}

func (l localDutyStore) Close() error {
	return l.store.Close()
}

// GAP-6-T1: a direct-source Duty with matching identity+family is detected
// by store.Conflict and by ReadConflict (the production chain's read path).
func TestGAP6DirectSourceIdentityAndFamilyCollision(t *testing.T) {
	t.Parallel()
	store, root, clock := openStoreWithClock(t)
	identity, family := [32]byte{11}, [32]byte{31}
	notAfter := clock().Add(time.Hour)
	if err := store.Replace([32]byte{200}, []localroles.Duty{{
		Identity: identity, Family: family,
		Class: "direct-source", State: "exposed", NotAfter: notAfter,
	}}); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	if conflict, err := store.Conflict(identity, family); err != nil || !conflict {
		_ = store.Close()
		t.Fatalf("store.Conflict = %v, %v", conflict, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if conflict, err := localroles.ReadConflict(root, clock, identity, family); err != nil || !conflict {
		t.Fatalf("ReadConflict = %v, %v", conflict, err)
	}
}

// GAP-6-T2: an unrelated direct-source duty (different identity+family) is
// not detected; both Conflict and ReadConflict return false.
func TestGAP6DirectSourceNonCollision(t *testing.T) {
	t.Parallel()
	store, root, clock := openStoreWithClock(t)
	notAfter := clock().Add(time.Hour)
	if err := store.Replace([32]byte{200}, []localroles.Duty{{
		Identity: [32]byte{99}, Family: [32]byte{98},
		Class: "direct-source", State: "exposed", NotAfter: notAfter,
	}}); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	identity, family := [32]byte{11}, [32]byte{31}
	if conflict, err := store.Conflict(identity, family); err != nil || conflict {
		_ = store.Close()
		t.Fatalf("store.Conflict = %v, %v", conflict, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if conflict, err := localroles.ReadConflict(root, clock, identity, family); err != nil || conflict {
		t.Fatalf("ReadConflict = %v, %v", conflict, err)
	}
}

// GAP-6-T3: a direct-source duty that collides on Family only (different
// identity) is still detected by both Conflict and ReadConflict.
func TestGAP6DirectSourceFamilyOnlyCollision(t *testing.T) {
	t.Parallel()
	store, root, clock := openStoreWithClock(t)
	family := [32]byte{31}
	notAfter := clock().Add(time.Hour)
	if err := store.Replace([32]byte{200}, []localroles.Duty{{
		Identity: [32]byte{99}, Family: family,
		Class: "direct-source", State: "exposed", NotAfter: notAfter,
	}}); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	identity := [32]byte{11}
	if conflict, err := store.Conflict(identity, family); err != nil || !conflict {
		_ = store.Close()
		t.Fatalf("store.Conflict = %v, %v", conflict, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if conflict, err := localroles.ReadConflict(root, clock, identity, family); err != nil || !conflict {
		t.Fatalf("ReadConflict = %v, %v", conflict, err)
	}
}

// GAP-6-T4: an expired direct-source duty is not detected once the read
// clock has advanced past its NotAfter. Both Conflict and ReadConflict
// return false. (The duty is inserted with a short lifetime; we close the
// store and re-open via ReadConflict with an advanced clock.)
func TestGAP6DirectSourceExpiredExposure(t *testing.T) {
	t.Parallel()
	store, root, seedClock := openStoreWithClock(t)
	identity, family := [32]byte{11}, [32]byte{31}
	if err := store.Replace([32]byte{200}, []localroles.Duty{{
		Identity: identity, Family: family,
		Class: "direct-source", State: "exposed", NotAfter: seedClock().Add(time.Second),
	}}); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	advanced := seedClock().Add(2 * time.Second)
	readClock := func() time.Time { return advanced }
	if conflict, err := localroles.ReadConflict(root, readClock, identity, family); err != nil || conflict {
		t.Fatalf("expired ReadConflict = %v, %v", conflict, err)
	}
	reopened, err := localroles.Open(localroles.Config{Root: root, Clock: readClock})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if conflict, err := reopened.Conflict(identity, family); err != nil || conflict {
		t.Fatalf("expired store.Conflict = %v, %v", conflict, err)
	}
}

func TestServingSourceGuardSurvivesDeadlineAndUnrelatedReplacement(t *testing.T) {
	t.Parallel()
	seed := time.Unix(1_800_000_000, 0).UTC()
	now := seed
	clock := func() time.Time { return now }
	root := filepath.Join(t.TempDir(), "local-roles")
	store, err := localroles.Open(localroles.Config{Root: root, Clock: clock, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	identity, family := [32]byte{11}, [32]byte{31}
	if err := store.Replace([32]byte{1}, []localroles.Duty{{
		Identity: identity, Family: family, Class: "direct-source", State: "live", NotAfter: seed.Add(time.Second),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	now = seed.Add(2 * time.Second)
	if protected, err := localroles.ReadConflict(root, clock, identity, family); err != nil || !protected {
		t.Fatalf("serving Source after its stored deadline: protected=%t err=%v", protected, err)
	}
	store, err = localroles.Open(localroles.Config{Root: root, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Replace([32]byte{2}, []localroles.Duty{{
		Identity: [32]byte{12}, Family: [32]byte{32}, Class: "node-duty", State: "live", NotAfter: now.Add(time.Hour),
	}}); err != nil {
		t.Fatal(err)
	}
	if protected, err := store.Conflict(identity, family); err != nil || !protected {
		t.Fatalf("unrelated Replace pruned serving Source: protected=%t err=%v", protected, err)
	}
	if err := store.Remove([32]byte{1}); err != nil {
		t.Fatal(err)
	}
	if protected, err := store.Conflict(identity, family); err != nil || protected {
		t.Fatalf("serving Source after owner Remove: protected=%t err=%v", protected, err)
	}
}

func TestDirectSourcePredecessorsMayOverlapOnlyWithinProducer(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	store, err := localroles.Open(localroles.Config{
		Root: filepath.Join(t.TempDir(), "local-roles"), Clock: func() time.Time { return now }, Create: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first := localroles.Duty{Identity: [32]byte{11}, Family: [32]byte{31}, Class: "direct-source", State: "live", NotAfter: now.Add(time.Hour)}
	second := localroles.Duty{Identity: first.Identity, Family: [32]byte{32}, Class: "direct-source", State: "live", NotAfter: now.Add(time.Hour)}
	if err := store.Replace([32]byte{1}, []localroles.Duty{first, second}); err != nil {
		t.Fatalf("same-producer guarded predecessors: %v", err)
	}
	if err := store.Replace([32]byte{2}, []localroles.Duty{{
		Identity: [32]byte{12}, Family: second.Family, Class: "node-duty", State: "live", NotAfter: now.Add(time.Hour),
	}}); !errors.Is(err, localroles.ErrLocalRoleConflict) {
		t.Fatalf("cross-producer family collision = %v, want ErrLocalRoleConflict", err)
	}
}

func TestHeldServingSourceStillCountsAgainstInstallationCap(t *testing.T) {
	t.Parallel()
	seed := time.Unix(1_800_000_000, 0).UTC()
	now := seed
	store, err := localroles.Open(localroles.Config{
		Root: filepath.Join(t.TempDir(), "local-roles"), Clock: func() time.Time { return now }, Create: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	serving := localroles.Duty{Identity: [32]byte{1}, Family: [32]byte{101},
		Class: "direct-source", State: "live", NotAfter: seed.Add(time.Second)}
	if err := store.Replace([32]byte{1}, []localroles.Duty{serving}); err != nil {
		t.Fatal(err)
	}
	now = seed.Add(2 * time.Second)
	batch := make([]localroles.Duty, 0, 64)
	for index := 0; index < 64; index++ {
		batch = append(batch, localroles.Duty{Identity: [32]byte{byte(index + 2)}, Family: [32]byte{byte(index + 102)},
			Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour)})
	}
	if err := store.Replace([32]byte{2}, batch); !errors.Is(err, localroles.ErrInstallationSourceExhausted) {
		t.Fatalf("expired-clock live guard was not counted: %v", err)
	}
	if protected, err := store.Conflict(serving.Identity, serving.Family); err != nil || !protected {
		t.Fatalf("failed over-cap replacement lost serving guard: protected=%t err=%v", protected, err)
	}
	if err := store.Remove([32]byte{1}); err != nil {
		t.Fatal(err)
	}
	if err := store.Replace([32]byte{2}, batch); err != nil {
		t.Fatalf("cap was not freed by owner removal: %v", err)
	}
}
