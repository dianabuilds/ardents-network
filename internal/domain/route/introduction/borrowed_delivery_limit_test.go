package introduction

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/route/ardp"
)

func TestIndependentDistinctSeventeenthBorrowRefused(t *testing.T) {
	r, _ := deliveryTransportFixture(t)
	envelope := deliveryEnvelope(t)
	for i := 0; i < 16; i++ {
		r.mu.Lock()
		r.deliveryStarts = nil
		r.mu.Unlock()
		body, err := EncodeDelivery([32]byte{byte(100 + i)}, envelope)
		if err != nil {
			t.Fatal(err)
		}
		if err = r.receiveDelivery(ardp.Frame{Kind: ardp.KindOperation, Lane: uint32(2 * (i + 1)), Body: body}); err != nil {
			t.Fatal(i, err)
		}
		if _, err = r.NextDelivery(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	r.mu.Lock()
	r.deliveryStarts = nil
	r.mu.Unlock()
	body, err := EncodeDelivery([32]byte{200}, envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.receiveDelivery(ardp.Frame{Kind: ardp.KindOperation, Lane: 34, Body: body}); err == nil {
		t.Fatal("distinct seventeenth borrowed delivery accepted")
	}
}
