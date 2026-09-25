package node

import (
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

func TestClosedForwardingReceivingRollbackRetainsInitializationAndCleanupFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		fail func(*closedForwardingReceivingOpeners, error)
	}{
		{
			name: "limits",
			fail: func(openers *closedForwardingReceivingOpeners, initial error) {
				openers.newLimits = func(func() time.Time) (*route.ClosedDutyLimits, error) { return nil, initial }
			},
		},
		{
			name: "bootstrap",
			fail: func(openers *closedForwardingReceivingOpeners, initial error) {
				openers.newBootstrap = func(func() time.Time) (*route.ClosedBootstrapController, error) { return nil, initial }
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			binding := replay.Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2},
				ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
			initial := errors.New("fixture initialization failed")
			cleanup := errors.New("fixture spend cleanup failed")
			closes := 0
			openers := defaultClosedForwardingReceivingOpeners()
			openers.closeSpends = func(spends *replay.Ledger) error {
				closes++
				return errors.Join(spends.Close(), cleanup)
			}
			test.fail(&openers, initial)

			resources, err := openClosedForwardingReceivingResourcesWith(root, binding, time.Now, openers)
			if resources != nil {
				t.Cleanup(func() { _ = resources.Close() })
			}
			if resources != nil || !errors.Is(err, initial) || !errors.Is(err, cleanup) {
				t.Fatalf("rollback = resources %p error %v", resources, err)
			}
			if closes != 1 {
				t.Fatalf("spend lease closed %d times", closes)
			}
			reopened, err := replay.Open(root, binding)
			if err != nil {
				t.Fatalf("rollback retained spend root: %v", err)
			}
			if err := reopened.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClosedForwardingReceivingTransfersOnlyCompleteResources(t *testing.T) {
	root := t.TempDir()
	binding := replay.Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2},
		ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	resources, err := openClosedForwardingReceivingResources(root, binding, time.Now)
	if resources != nil {
		t.Cleanup(func() { _ = resources.Close() })
	}
	if err != nil {
		t.Fatal(err)
	}
	if resources.spends == nil || resources.limits == nil || resources.bootstrap == nil {
		t.Fatal("incomplete receiving resources transferred")
	}
	for range 2 {
		if err := resources.Close(); err != nil {
			t.Fatal(err)
		}
	}
	reopened, err := replay.Open(root, binding)
	if err != nil {
		t.Fatalf("closed owner retained spend root: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}
