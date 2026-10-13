package prefix

import (
	"bytes"
	"context"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
)

func TestBootstrapFlightPreservesCanonicalRecords(t *testing.T) {
	// Literal independent wire layout: HELLO, then issuer-only BOOTSTRAP.
	// This checks encoding only; the old timestamp grants no live authority.
	body := strings.Repeat("01", 160) + "000000000000000107" + strings.Repeat("02", 32) + "0000000000000001"
	raw, err := hex.DecodeString(body)
	if err != nil {
		t.Fatal(err)
	}
	hello, err := ardp.DecodeHello(raw)
	if err != nil {
		t.Fatal(err)
	}
	wanted, err := hex.DecodeString("415244500003010000000000000000d1" + body + "4152445000030300000000000000000102")
	if err != nil {
		t.Fatal(err)
	}
	flight, err := bootstrapFlight(hello)
	if err != nil || !bytes.Equal(flight, wanted) {
		t.Fatalf("bootstrap flight changed canonical records: %x, %v", flight, err)
	}
	hello.Purpose = 8
	if flight, err := bootstrapFlight(hello); err == nil || len(flight) != 0 {
		t.Fatal("invalid HELLO produced a partial flight", err)
	}
}

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
