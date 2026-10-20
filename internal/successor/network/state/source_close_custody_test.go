package state_test

import (
	"crypto/sha256"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// The original serving producer is derived from the State root. A successor
// must not reacquire that root while an older Close can still remove its guard.
// The existing clock seam holds the actual guard-opening boundary; no native
// lock, successful State result, or physical listener join is substituted.
func TestServingSourceRetainsRootThroughGuardRemoval(t *testing.T) {
	genesis := newFixture(t)
	config := installedServingSourceConfig(t, genesis)
	now := config.Now
	var armed atomic.Bool
	entered, release := make(chan struct{}), make(chan struct{})
	config.Now = time.Time{}
	config.Clock = func() time.Time {
		if armed.CompareAndSwap(true, false) {
			close(entered)
			<-release
		}
		return now
	}
	original, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := original.Current()
	if err != nil {
		_ = original.Close()
		t.Fatal(err)
	}
	armed.Store(true)
	joined := make(chan error, 1)
	go func() { joined <- original.Close() }()
	<-entered
	successorConfig := config
	successorConfig.Clock = func() time.Time { return now }
	successor, reopenErr := state.Open(successorConfig)
	close(release)
	originalErr := <-joined
	if originalErr != nil {
		t.Fatal("original Source Close failed", originalErr)
	}
	family := sha256.Sum256([]byte(snapshot.SourceFamily))
	if reopenErr == nil {
		protected, conflictErr := duty.ReadConflict(config.LocalRoleStateRoot, successorConfig.Clock, snapshot.SourceNodeID, family)
		closeErr := successor.Close()
		t.Fatalf("successor acquired State before original guard removal joined: guard=%t conflict=%v successor-close=%v", protected, conflictErr, closeErr)
	}
	successor, err = state.Open(successorConfig)
	if err != nil {
		t.Fatal("joined original did not release State", err)
	}
	protected, conflictErr := duty.ReadConflict(config.LocalRoleStateRoot, successorConfig.Clock, snapshot.SourceNodeID, family)
	if closeErr := successor.Close(); conflictErr != nil || !protected || closeErr != nil {
		t.Fatal("successor guard or Close failed", protected, conflictErr, closeErr)
	}
	if protected, err := duty.ReadConflict(config.LocalRoleStateRoot, successorConfig.Clock, snapshot.SourceNodeID, family); err != nil || protected {
		t.Fatal("joined successor retained guard", protected, err)
	}
}
