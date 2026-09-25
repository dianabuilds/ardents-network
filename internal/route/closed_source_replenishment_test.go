//go:build linux

package route

import (
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func TestClosedSourceReplenishmentConsumesLaneZeroAccept(t *testing.T) {
	owner, peer, _ := sourceChannelsFixture(t)
	owner.mu.Lock()
	owner.transferred = closedRefillThreshold
	owner.mu.Unlock()
	hello := ardp.Hello{ChannelNonce: [32]byte{1}, Deadline: time.Now().Add(time.Minute)}
	served := make(chan error, 1)
	go func() {
		request, err := ardp.ReadFrame(peer)
		if err == nil && (request.Kind != ardp.KindAdmit || request.Lane != 0) {
			err = errors.New("unexpected refill request")
		}
		if err == nil {
			var accepted ardp.Frame
			accepted, err = ardp.AcceptFrame(0, 64<<10)
			if err == nil {
				err = ardp.WriteFrame(peer, accepted)
			}
		}
		served <- err
	}()
	if err := owner.replenish(t.Context(), hello, func(ardp.Hello, uint8) ([]byte, error) {
		return make([]byte, 354), nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := <-served; err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	base, pending := owner.refillBase, owner.refill != nil
	owner.mu.Unlock()
	if base != closedRefillThreshold || pending {
		t.Fatalf("refill completion = base %d pending %t", base, pending)
	}
}
