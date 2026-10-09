package introduction

import (
	"context"
	"errors"
	"testing"
	"time"
)

// These controls isolate refusal during the final observation; the fake
// checker supplies no successful REGISTER authority. Both-Carrier command
// scenarios independently verify genuine positive receipt production.
func TestReceiptRefusesLossDuringObservation(t *testing.T) {
	for _, loss := range []string{"stop", "withdraw", "caller", "expiry"} {
		t.Run(loss, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			r := &HolderRegistration{ctx: ctx}
			r.receipt = Receipt{owner: r, facts: RegistrationFacts{Expiry: time.Now().Add(time.Minute)}}
			if loss == "expiry" {
				r.receipt.facts.Expiry = time.Now().Add(20 * time.Millisecond)
			}
			r.check = func() error {
				r.mu.Lock()
				defer r.mu.Unlock()
				switch loss {
				case "stop":
					r.stopped = true
				case "withdraw":
					r.withdrawing = true
				case "caller":
					cancel()
				case "expiry":
					<-time.After(time.Until(r.receipt.facts.Expiry))
				}
				return nil
			}
			receipt, err := r.Receipt()
			if err == nil || receipt != (Receipt{}) {
				t.Fatal("late observation created receipt", receipt, err)
			}
		})
	}
}

func TestReceiptRejectsForeignAndDetachedFacts(t *testing.T) {
	r := &HolderRegistration{ctx: t.Context()}
	r.receipt = Receipt{owner: r, facts: RegistrationFacts{Revision: 1, Expiry: time.Now().Add(time.Minute)}}
	calls := 0
	r.check = func() error { calls++; return errors.New("original observation refused") }
	foreign := &HolderRegistration{}
	for _, receipt := range []Receipt{{}, {owner: foreign}, {owner: r, facts: r.receipt.facts}} {
		if receipt == r.receipt {
			receipt.facts.Revision++
		}
		if err := r.CheckReceipt(receipt); err == nil {
			t.Fatal("foreign/detached receipt accepted")
		}
	}
	if calls != 0 {
		t.Fatal("foreign receipt reached observation", calls)
	}
	if _, err := r.Receipt(); err == nil || calls != 1 {
		t.Fatal("lost original observation accepted", err, calls)
	}
	facts := r.receipt.Facts()
	facts.Revision++
	if r.receipt.Facts().Revision != 1 {
		t.Fatal("detached facts changed original receipt")
	}
}
