//go:build linux

package receiver

import (
	"context"
	"errors"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"sync/atomic"
	"testing"
	"time"
)

func TestAdmissionRollbackRetainsCapacityUntilPhysicalJoin(t *testing.T) {
	physical := newLifecycleConn(false)
	physical.readGate = make(chan struct{})
	s := framing.New(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, false, framing.NewBudget(64<<20), nil)
	lifetime := &admissionRetirement{}
	releaseFailure := errors.New("retained capacity release failed")
	var releases atomic.Int32
	rollback, err := (Channel{capacity: lifetime}).HoldReservation(func() error {
		select {
		case <-s.Done():
		default:
			t.Error("capacity released before physical reader joined")
		}
		releases.Add(1)
		return releaseFailure
	})
	if err != nil {
		t.Fatal(err)
	}
	// Receiving Admission has refused after acquiring capacity; no successful
	// Grant is invented by this lifecycle oracle.
	if err := rollback(); err != nil {
		t.Fatal(err)
	}
	if releases.Load() != 0 {
		t.Fatal("rollback bypassed physical retirement")
	}
	result := make(chan error, 1)
	go func() { result <- errors.Join(s.Close(), lifetime.finish()) }()
	<-physical.closed
	if releases.Load() != 0 {
		t.Fatal("socket Close substituted for reader join")
	}
	close(physical.readGate)
	if err := lifecycleResult(t, result); !errors.Is(err, releaseFailure) {
		t.Fatal("joined release failure lost", err)
	}
	if err := rollback(); !errors.Is(err, releaseFailure) || releases.Load() != 1 {
		t.Fatal("repeated release changed result or retried mutation", err)
	}
}

func TestAdmissionRetirementReturnsAdditionsBeforeOriginal(t *testing.T) {
	lifetime := &admissionRetirement{}
	channel := Channel{capacity: lifetime}
	var order []int
	var releases []func() error
	for i := range 3 {
		release, err := channel.HoldReservation(func() error {
			order = append(order, i)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	for _, release := range releases {
		if err := release(); err != nil {
			t.Fatal(err)
		}
	}
	if len(order) != 0 {
		t.Fatal("capacity returned before join", order)
	}
	if err := lifetime.finish(); err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 || order[0] != 2 || order[1] != 1 || order[2] != 0 {
		t.Fatal("original termination returned before additions", order)
	}
	if err := lifetime.finish(); err != nil || len(order) != 3 {
		t.Fatal("repeat join returned capacity twice", order, err)
	}
}
