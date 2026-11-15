package forwarding

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/nodeprincipal"
)

func TestNodePrincipalPreservesOriginalCaller(t *testing.T) {
	for _, expired := range []bool{false, true} {
		name := "canceled"
		if expired {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			var ctx context.Context
			var cancel context.CancelFunc
			if expired {
				ctx, cancel = context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
			} else {
				ctx, cancel = context.WithCancel(t.Context())
				cancel()
			}
			defer cancel()
			if err := observeNodePrincipal(ctx, 997, 997); err != ctx.Err() {
				t.Fatal("original caller error replaced without an opened file", err)
			}
		})
	}
}

func TestPrincipalObservationRetainsRoleAndPhysicalFailures(t *testing.T) {
	physical := errors.New("physical close")
	err := nodeprincipal.MapObservationError(errors.Join(nodeprincipal.ErrBinding, physical), ErrInput, ErrBinding)
	if !errors.Is(err, ErrBinding) || !errors.Is(err, physical) || errors.Is(err, nodeprincipal.ErrBinding) {
		t.Fatalf("observation failures changed: %v", err)
	}
}
