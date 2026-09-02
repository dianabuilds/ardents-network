package duty_test

import (
	"path/filepath"
	"testing"
	"time"

	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
)

// GAP-6 regression tests for the source-exposure ledger consulted at Entry
// admission.
//
// The contract placed these tests in `internal/entry/entry_test.go` with a
// real `duty.Store` and a real `entry.Verify` call. The package-map rule
// (enforced by `internal/architecture`) forbids `internal/entry` from
// importing `internal/network/duty`, and `internal/network/duty` from
// importing `internal/entry`. We therefore place the tests here, where a
// real `duty.Store` is constructible, and demonstrate the duty side of the
// production chain: a `direct-source` duty retained by `Replace` is
// detected by `store.Conflict` and by `ReadConflict`, the function
// production wires as `entry.Verification.Conflict`.
//
// The contract's four behaviours are exercised end-to-end on the store
// side. The entry-side wiring (`Verify` calling `Conflict` and returning
// `ConflictingRole`) is unchanged production code and is covered by the
// existing `internal/entry/entry_test.go` test
// `TestVerifyReturnsOnlyCurrentInitiatorAuthorization`.

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
