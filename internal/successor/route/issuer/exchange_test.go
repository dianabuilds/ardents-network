package issuer

import (
	"bytes"
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// This framing refusal supplies no successful authority, permission or signature.
func TestServeRejectsMalformedOperationBeforeIssuing(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := peer.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	finished := make(chan error, 1)
	go func() {
		finished <- Serve(ctx, local, ardp.Hello{Purpose: ardp.PurposeIssuer, Deadline: time.Now().Add(time.Second)}, 64<<10, ctx.Err, func(context.Context, []byte) ([]byte, error) {
			calls.Add(1)
			return nil, errors.New("unexpected issuing effect")
		})
	}()
	body := make([]byte, ardp.IssuerBodySize)
	body[0], body[1] = 255, 1
	if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindOperation, Body: body}); err != nil {
		cancel()
		<-finished
		t.Fatal(err)
	}
	if err := <-finished; err == nil || calls.Load() != 0 {
		t.Fatal("malformed operation reached issuing", err, calls.Load())
	}
}

func TestIssuerResultStatusMatchesAdmissionOutcome(t *testing.T) {
	for _, pair := range []struct {
		admission admission.ClosedTokenBatchStatus
		wire      uint8
	}{
		{admission.ClosedTokenIssued, 0}, {admission.ClosedTokenExhausted, 2},
		{admission.ClosedTokenWithdrawn, 4}, {admission.ClosedTokenUnavailable, 1},
	} {
		got, err := resultStatus(pair.admission)
		if err != nil || got != pair.wire {
			t.Fatal("issuer outcome changed", pair, got, err)
		}
	}
	for _, invalid := range []admission.ClosedTokenBatchStatus{0, 5, 255} {
		if _, err := resultStatus(invalid); err == nil {
			t.Fatal("unknown issuer outcome accepted", invalid)
		}
	}
}

func TestIssuerResultRejectsContradictoryEnvelopeAndPadding(t *testing.T) {
	// These synthetic canonical signatures test framing only. They do not
	// satisfy Stock's independent cryptographic verification or mint a token.
	for _, pair := range []struct {
		admission admission.ClosedTokenBatchStatus
		wire      uint8
	}{
		{admission.ClosedTokenIssued, 0}, {admission.ClosedTokenExhausted, 2},
		{admission.ClosedTokenWithdrawn, 4}, {admission.ClosedTokenUnavailable, 1},
	} {
		outcome := admission.ClosedTokenBatchResult{Status: pair.admission}
		if pair.admission == admission.ClosedTokenIssued {
			outcome.Signatures = [][]byte{bytes.Repeat([]byte{7}, admission.BlindSignatureSize)}
		}
		payload, err := admission.EncodeClosedTokenBatchResult(outcome)
		if err != nil {
			t.Fatal(err)
		}
		for status := uint8(0); status <= 5; status++ {
			err := checkResult(status, payload)
			if (err == nil) != (status == pair.wire) {
				t.Fatal("contradictory issuer outcome", pair, status, err)
			}
		}
		payload[len(payload)-1] = 1
		if err := checkResult(pair.wire, payload); err == nil {
			t.Fatal("issuer padding accepted")
		}
		clear(payload)
	}
}
