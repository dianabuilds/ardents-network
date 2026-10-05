package prefix

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
)

// This portable composition refusal test consumes the actual parent exchange. It supplies
// no successful token, State or grant: genuine holder presentation and durable
// spend are exercised by the command's both-Carrier integration tests.
func TestParentPresentationRefusesWrongPurposeAndTokenBeforeOutput(t *testing.T) {
	for _, test := range []struct {
		name      string
		purpose   ardp.Purpose
		length    int
		presented bool
	}{
		{"wrong-purpose", ardp.PurposeDataJoin, 354, false},
		{"short-token", ardp.PurposeForwarding, 353, true},
		{"long-token", ardp.PurposeForwarding, 355, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			physical := newLifecycleConn(false)
			end := time.Now().Add(time.Minute)
			s := framing.New(t.Context(), physical, end, 400, nil, false, framing.NewBudget(4<<20), nil)
			defer s.Close()
			called := false
			err := replenishParent(s, t.Context(), t.Context(), ardp.Hello{Purpose: test.purpose, Deadline: end}, func(context.Context, ardp.Hello) ([]byte, error) {
				called = true
				return make([]byte, test.length), nil
			})
			if err == nil || called != test.presented {
				t.Fatal("invalid request crossed presentation boundary", err, called)
			}
			select {
			case <-physical.writes:
				t.Fatal("invalid request started physical output")
			default:
			}
			if !s.Live() {
				t.Fatal("pre-effect refusal retired the original parent")
			}
		})
	}
}
