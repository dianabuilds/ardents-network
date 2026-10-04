package state_test

import (
	"context"
	"crypto/sha256"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

func TestSourceWaveKeepsContactGuardPastJournalDeadline(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config, closeSources := sourceEnvironment(t, genesis, successor, successor)
	defer closeSources()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	config.Source.Sources[0].Address = listener.Addr().String()
	config.Source.Sources[1].Address = "127.0.0.1:1"
	var unixClock atomic.Int64
	unixClock.Store(genesis.now)
	clock := func() time.Time { return time.Unix(unixClock.Load(), 0).UTC() }
	config.Clock = clock
	config.ObserveClock = clock
	serving, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := serving.Close(); err != nil {
			t.Error(err)
		}
	}()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, refreshErr := serving.Refresh(ctx); finished <- refreshErr }()
	accepted := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- connection
	}()
	var blocked net.Conn
	select {
	case blocked = <-accepted:
		defer blocked.Close()
	case err := <-acceptErr:
		t.Fatalf("accept blocked Source contact: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("Source wave never contacted the blocked Source")
	}
	unixClock.Store(genesis.now + 16)
	identity := config.Source.Sources[0].Identity
	family := sha256.Sum256([]byte(config.Source.Sources[0].Family))
	if protected, err := duty.ReadConflict(config.LocalRoleStateRoot, clock, identity, family); err != nil || !protected {
		t.Fatalf("in-flight Source after journal deadline: protected=%t err=%v", protected, err)
	}
	cancel()
	_ = blocked.Close()
	select {
	case refreshErr := <-finished:
		if refreshErr == nil {
			t.Fatal("canceled wave unexpectedly accepted an Epoch")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled Source wave did not join")
	}
	unixClock.Store(genesis.now + 30*60 + 1)
	if protected, err := duty.ReadConflict(config.LocalRoleStateRoot, clock, identity, family); err != nil || protected {
		t.Fatalf("joined failed wave retained contact guard past the current Epoch: protected=%t err=%v", protected, err)
	}
}

func TestInterruptedSourceWaveKeepsGuardUntilVerifiedReopen(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config, closeSources := sourceEnvironment(t, genesis, successor, successor)
	defer closeSources()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	config.Source.Sources[0].Address = listener.Addr().String()
	config.Source.Sources[1].Address = "127.0.0.1:1"
	var unixClock atomic.Int64
	unixClock.Store(genesis.now)
	clock := func() time.Time { return time.Unix(unixClock.Load(), 0).UTC() }
	config.Clock, config.ObserveClock = clock, clock
	opened, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { _, refreshErr := opened.Refresh(t.Context()); finished <- refreshErr }()
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- connection
		}
	}()
	var blocked net.Conn
	select {
	case blocked = <-accepted:
		defer blocked.Close()
	case <-time.After(3 * time.Second):
		t.Fatal("Source wave never contacted the blocked Source")
	}
	unixClock.Store(genesis.now + 30*60 + 1)
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("interrupted Source wave did not join")
	}
	identity := config.Source.Sources[0].Identity
	if held, err := duty.ReadConflict(config.LocalRoleStateRoot, clock, identity, [32]byte{}); err != nil || !held {
		t.Fatalf("interrupted journal lost its conservative guard: held=%t err=%v", held, err)
	}
	reopened, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if held, err := duty.ReadConflict(config.LocalRoleStateRoot, clock, identity, [32]byte{}); err != nil || held {
		t.Fatalf("verified reopen retained a stale work-only guard: held=%t err=%v", held, err)
	}
}
