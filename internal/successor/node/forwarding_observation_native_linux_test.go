//go:build linux && node_process_native

package node

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// This component requires the real sole-process Node native envelope. Genuine
// Guard and participation observations supply local admission only. A held
// foreign refusal cannot grant State authority, Route admission or Node join.
func TestForwardingNativeLocalProtectionDuringHeldAuthority(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	guard, err := processresource.Open(ctx, processresource.Profile)
	if err != nil {
		t.Fatal("selected native process environment unavailable", err)
	}
	defer func() {
		if err := guard.Close(); err != nil {
			t.Error("original Guard close", err)
		}
	}()
	now := time.Now()
	producer, err := duty.OpenNodeProducer(ctx, duty.Config{Root: filepath.Join(t.TempDir(), "participation"), Clock: time.Now, Create: true}, duty.NodeParticipation{Identity: [32]byte{1}, FamilyID: [32]byte{2}, Assignment: "initiator", EpochUntil: now.Add(time.Minute), RecordUntil: now.Add(time.Minute)})
	if err != nil {
		t.Fatal("original producer", err)
	}
	defer func() {
		if err := producer.Close(context.Background()); err != nil {
			t.Error("original producer close", err)
		}
	}()
	if err := producer.Live(); err != nil {
		t.Fatal(err)
	}
	entered, resume := make(chan struct{}), make(chan struct{})
	foreign := errors.New("held foreign refusal")
	f := &forwarding{ctx: ctx, cancel: cancel, process: guard, producer: producer, retired: errors.New("original retirement"), config: ForwardingConfig{Authority: role.Authority{Current: func() (network.RuntimeView, error) {
		close(entered)
		<-resume
		return network.RuntimeView{}, foreign
	}}}}
	joined := make(chan struct{})
	var observed error
	go func() { _, observed = f.current(); close(joined) }()
	defer func() {
		close(resume)
		<-joined
		if observed != foreign {
			t.Error("original foreign refusal lost", observed)
		}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("full observation did not enter foreign boundary")
	}
	if _, err := f.checkLocal(); err != nil {
		t.Fatal("held foreign read blocked independent actual local checks", err)
	}
	// An actual runtime placement violation must still deny the invocation
	// while its foreign borrower is held; restoring placement cannot revive it.
	previous := runtime.GOMAXPROCS(2)
	_, refusal := f.checkLocal()
	runtime.GOMAXPROCS(previous)
	if refusal == nil || processresource.Failure(refusal) == nil {
		t.Fatal("actual changed process placement escaped local protection", refusal)
	}
	if ctx.Err() == nil || f.first != refusal || f.firstOwner != "process" {
		t.Fatal("local failure did not cancel/retain the original owner")
	}
	if _, err := f.checkLocal(); err != f.retired {
		t.Fatal("restored placement revived original local admission", err)
	}
	select {
	case <-joined:
		t.Fatal("held foreign physical borrower was declared complete")
	default:
	}
}
