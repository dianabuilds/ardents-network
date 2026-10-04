//go:build linux

package introduction

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistrationCapacityAccountsPendingBeforeAnyClaim(t *testing.T) {
	history, _, root, _ := newHistoryFixture(t)
	registry, err := NewRegistry(history)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0).UTC()
	before, err := os.ReadFile(filepath.Join(root, slotHistoryName))
	if err != nil {
		t.Fatal(err)
	}
	positions := make([]*Capacity, 0, 1024)
	for range 1024 {
		position, err := registry.Reserve(now)
		if err != nil {
			t.Fatal(err)
		}
		positions = append(positions, position)
		defer position.Release()
	}
	if _, err := registry.Reserve(now); err == nil {
		t.Fatal("pending registrations oversubscribed duty")
	}
	positions[0].Release()
	positions[0].Release()
	last, err := registry.Reserve(now)
	if err != nil {
		t.Fatal("one joined capacity return not observed", err)
	}
	defer last.Release()
	if _, err := registry.Reserve(now); err == nil {
		t.Fatal("repeated return refunded another position")
	}
	after, err := os.ReadFile(filepath.Join(root, slotHistoryName))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("capacity reservation wrote durable claim", err)
	}
}

func TestRegistrationReservesWithdrawalBeforeDurableClaim(t *testing.T) {
	history, _, root, _ := newHistoryFixture(t)
	registry, err := NewRegistry(history)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0).UTC()
	position, err := registry.Reserve(now)
	if err != nil {
		t.Fatal(err)
	}
	defer position.Release()
	request := Request{Nonce: [32]byte{1}, Slot: [32]byte{2}, Revision: 3, Expiry: now.Add(time.Minute)}
	before, err := os.ReadFile(filepath.Join(root, slotHistoryName))
	if err != nil {
		t.Fatal(err)
	}
	// 617 admission bytes + two complete 20512-byte exchanges = 41641.
	// The older ordering checked only REGISTER and added withdrawal afterward.
	if _, err := registry.Register(position, request, [32]byte{4}, 41640, request.Expiry, 617, now); err == nil {
		t.Fatal("claim exceeded original withdrawal reserve")
	}
	after, err := os.ReadFile(filepath.Join(root, slotHistoryName))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("exhaustion touched durable slot history", err)
	}
	registration, err := registry.Register(position, request, [32]byte{4}, 41641, request.Expiry, 617, now)
	if err != nil {
		t.Fatal("exact original exchange bound refused", err)
	}
	if registration.used != 41641 {
		t.Fatal("actual ingress/egress+headers reserve differs")
	}
}

func TestRegistrationOwningWithdrawalAndLostAcknowledgementRetainFloor(t *testing.T) {
	for _, acknowledged := range []bool{false, true} {
		history, _, root, binding := newHistoryFixture(t)
		registry, err := NewRegistry(history)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Unix(1_800_000_000, 0).UTC()
		position, err := registry.Reserve(now)
		if err != nil {
			t.Fatal(err)
		}
		defer position.Release()
		request := Request{Nonce: [32]byte{1}, Slot: [32]byte{2}, Revision: 3, Expiry: now.Add(time.Minute)}
		channel := [32]byte{4}
		registration, err := registry.Register(position, request, channel, 1048576, request.Expiry, 617, now)
		if err != nil {
			t.Fatal(err)
		}
		withdraw := Request{Nonce: [32]byte{5}, Slot: request.Slot, Revision: 3, Withdraw: true}
		if !acknowledged {
			if err := registration.Withdraw(withdraw, channel, now); err == nil {
				t.Fatal("unacknowledged slot granted live withdrawal")
			}
		} else {
			if err := registration.Acknowledge(now); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"channel", "slot", "revision", "nonce", "register", "expiry"} {
				wrong, peer := withdraw, channel
				switch fault {
				case "channel":
					peer[0]++
				case "slot":
					wrong.Slot[0]++
				case "revision":
					wrong.Revision++
				case "nonce":
					wrong.Nonce = request.Nonce
				case "register":
					wrong.Withdraw = false
				case "expiry":
					wrong.Expiry = request.Expiry
				}
				if err := registration.Withdraw(wrong, peer, now); err == nil {
					t.Fatalf("accepted wrong %s", fault)
				}
			}
			if err := registration.Withdraw(withdraw, channel, now); err != nil {
				t.Fatal(err)
			}
			if err := registration.Withdraw(withdraw, channel, now); err == nil {
				t.Fatal("withdrawn handle reused")
			}
		}
		registration.Retire()
		registration.Retire()
		if err := registration.Acknowledge(now); err == nil {
			t.Fatal("retired handle acknowledged late")
		}
		// A genuinely new channel has its own unconsumed capacity. Reusing
		// position would refuse on transfer state and never test the slot floor.
		newPosition, err := registry.Reserve(now)
		if err != nil {
			t.Fatal(err)
		}
		defer newPosition.Release()
		if _, err := registry.Register(newPosition, request, [32]byte{9}, 1048576, request.Expiry, 617, now); err == nil {
			t.Fatal("retirement/new channel reclaimed original slot")
		}
		if err := registry.Close(); err != nil {
			t.Fatal(err)
		}
		reopened, err := OpenHistory(root, binding)
		if err != nil {
			t.Fatal(err)
		}
		if err := reopened.Claim(request.Slot, request.Expiry, now.Add(time.Second)); err == nil {
			t.Fatal("withdrawal/lost ACK refunded durable slot")
		}
		if err := reopened.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
