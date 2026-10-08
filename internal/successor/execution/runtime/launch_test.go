package runtime

import (
	"context"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

func TestPreparationRefusesBeforeActivationWithoutExactLiveGrant(t *testing.T) {
	owner, err := New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: [32]byte{2}, Surface: execution.Connection}}})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, test := range []struct {
		ctx       context.Context
		principal [32]byte
		surface   execution.Surface
	}{
		{ctx, [32]byte{2}, execution.Connection},
		{t.Context(), [32]byte{3}, execution.Connection},
		{t.Context(), [32]byte{2}, execution.Administration},
	} {
		if prepared, err := owner.Prepare(test.ctx, test.principal, test.surface); prepared != nil || err == nil {
			t.Fatal("unadmitted preparation launched")
		}
	}
	if len(launchGate) != 0 {
		t.Fatal("pre-admission refusal touched fixed activation inventory")
	}
}
