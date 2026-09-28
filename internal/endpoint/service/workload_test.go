//go:build linux

package service

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

func TestTextServiceWorkloadBoundsPreserveCurrentDirectionalContracts(t *testing.T) {
	for _, test := range []struct {
		name                   string
		open                   func() (WorkloadBounds, error)
		readerSend, readerRead uint32
	}{
		{name: "document", open: DocumentWorkloadBounds,
			readerSend: 512, readerRead: textdocument.MaximumBytes + 13},
		{name: "qualification", open: StreamQualificationWorkloadBounds,
			readerSend: 64 << 20, readerRead: 64 << 20},
	} {
		t.Run(test.name, func(t *testing.T) {
			bounds, err := test.open()
			if err != nil {
				t.Fatal(err)
			}
			for _, direction := range []struct {
				name          string
				surface       broker.Surface
				send, receive uint32
			}{
				{name: "reader", surface: broker.Connection, send: test.readerSend, receive: test.readerRead},
				{name: "publisher", surface: broker.Administration, send: test.readerRead, receive: test.readerSend},
			} {
				t.Run(direction.name, func(t *testing.T) {
					send, receive, err := bounds.Direction(direction.surface)
					if err != nil || send != direction.send || receive != direction.receive {
						t.Fatalf("directional bounds = %d/%d, %v; want %d/%d", send, receive, err, direction.send, direction.receive)
					}
				})
			}
		})
	}
}

func TestTextServiceWorkloadBoundsRejectUncheckedValues(t *testing.T) {
	for _, test := range []struct {
		name          string
		send, receive uint32
	}{
		{name: "missing-send", receive: 1},
		{name: "missing-receive", send: 1},
		{name: "send-too-large", send: maximumStreamBytes + 1, receive: 1},
		{name: "receive-too-large", send: 1, receive: maximumStreamBytes + 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewWorkloadBounds(test.send, test.receive); err == nil {
				t.Fatal("unchecked text Service workload bounds accepted")
			}
		})
	}
	if _, _, err := (WorkloadBounds{}).Direction(broker.Connection); err == nil {
		t.Fatal("missing text Service workload contract accepted")
	}
	valid := mustWorkloadBounds(t, 1, 1)
	if _, _, err := valid.Direction(broker.Surface("unknown")); err == nil {
		t.Fatal("unknown text Service workload direction accepted")
	}
}

func mustWorkloadBounds(t *testing.T, send, receive uint32) WorkloadBounds {
	t.Helper()
	bounds, err := NewWorkloadBounds(send, receive)
	if err != nil {
		t.Fatal(err)
	}
	return bounds
}
