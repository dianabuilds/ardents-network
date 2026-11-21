package reachability

import (
	"errors"
	"os"
	"path/filepath"
)

// RepairRetainedStoreStage clears only the uncertain staging file of an
// EXISTING retained Store root, under its exclusive lease, after validating
// every committed floor. It is the bounded Store half of the platform-led
// Descriptor participation repair: the repair adapter calls it only after
// independently proving the original service defunct (real failed-unit manager
// facts and an empty per-unit cgroup) and while holding the participation lease
// and State reader. This mechanism itself grants no publication, lookup or
// runtime authority.
//
// It never initializes a missing root, marker, lock or records directory, never
// resets or evicts a committed floor, never promotes staging into a committed
// record and never deletes foreign entries. Staging is uncommitted (never a
// committed proof, including a complete synced write), so its removal loses no
// committed publication. With no staging present it is a validating no-op.
// OpenStore and OpenRetainedStore behavior is unchanged: a fresh opening still
// removes staging without a dead-unit proof, and a retained opening still
// refuses it.
func RepairRetainedStoreStage(config StoreConfig) (result error) {
	if !storePlatformSupported || config.Root == "" || config.Network == [32]byte{} {
		return errors.New("reachability Store configuration or platform unavailable")
	}
	root, err := filepath.Abs(config.Root)
	if err != nil {
		return err
	}
	// Retained-style existing-root requirement: a missing root is never
	// initialized by repair.
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.Join(errors.New("reachability retained root missing or invalid"), err)
	}
	originalRoot, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	lease, err := acquireStoreLease(originalRoot)
	if err != nil {
		return errors.Join(err, originalRoot.Close())
	}
	defer func() { result = errors.Join(result, lease.release()) }()
	lockInfo, err := originalRoot.Lstat(storeLockName)
	if err == nil {
		err = lease.check(lockInfo)
	}
	if err == nil {
		err = checkRetainedStoreRoot(originalRoot)
	}
	if err != nil {
		return errors.Join(err, originalRoot.Close())
	}
	custody, err := retainStoreCustody(originalRoot)
	if err != nil {
		// retainStoreCustody already joined the original root on failure.
		return err
	}
	defer func() { result = errors.Join(result, custody.close()) }()
	// Validate every committed floor before touching the interrupted
	// transaction, using the same closed grammar as a live opening. The repair
	// selects stage removal only here, after the caller's dead-unit proofs,
	// while this exclusive lease fences any new writer; the removal is synced
	// under the same lease before custody and lease release.
	floors := make(map[[32]byte]retainedFloor)
	return restoreStoreFloors(custody, config.Network, floors, storeStageRemove)
}
