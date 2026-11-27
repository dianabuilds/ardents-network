package transfer

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

func TestMeasuredReplayAndUnreadQueuesPreserveBytesAcrossReplacement(t *testing.T) {
	capture, err := measurement.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer capture.Stop()
	sender, err := newSendLedger(1, 32)
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := newReceiveLedger(1, 32)
	if err != nil {
		t.Fatal(err)
	}
	current := func(id runtimeevent.Instrument) uint64 {
		t.Helper()
		capture.Sample()
		spec, _ := id.Spec()
		for _, v := range capture.Latest().Values {
			if v.Name == spec.Name {
				if !v.Available {
					t.Fatal(v)
				}
				return v.Current
			}
		}
		t.Fatal("queue unobserved")
		return 0
	}
	if err := sender.accept([]byte("secret")); err != nil {
		t.Fatal(err)
	}
	ticket, err := sender.beginWrite()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sender.finishWrite(ticket, nil); err != nil {
		t.Fatal(err)
	}
	if err := sender.replace(2, 2); err != nil {
		t.Fatal(err)
	}
	if current(runtimeevent.ConnectionSendBytes) != 4 {
		t.Fatal("replacement erased replay queue")
	}
	value := wire.Data{AttachmentGeneration: 1, Offset: 0, Payload: []byte("secret")}
	if err := receiver.accept(value); err != nil {
		t.Fatal(err)
	}
	if err := receiver.accept(value); err != nil {
		t.Fatal("duplicate refused", err)
	}
	if current(runtimeevent.ConnectionReceiveBytes) != 6 {
		t.Fatal("duplicate changed unread bytes")
	}
	data := make([]byte, 2)
	if n, err := receiver.read(data); n != 2 || err != nil {
		t.Fatal(n, err)
	}
	if current(runtimeevent.ConnectionReceiveBytes) != 4 {
		t.Fatal("read cursor not reflected")
	}
}

func TestRejectedReplacementDoesNotTrimMeasuredReplay(t *testing.T) {
	capture, err := measurement.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer capture.Stop()
	state, err := New(1, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Write([]byte("request")); err != nil {
		t.Fatal(err)
	}
	_, ticket, err := state.BeginRecordWrite()
	if err != nil {
		t.Fatal(err)
	}
	if err := state.FinishRecordWrite(ticket, nil); err != nil {
		t.Fatal(err)
	}
	if next, err := state.PrepareReplacement(2, wire.Continuity{ReceiveNext: 2, SendBase: 1, SendEnd: 1}); err == nil || next != nil {
		t.Fatal("invalid peer range passed replacement preparation", err)
	}
	spec, _ := runtimeevent.ConnectionSendBytes.Spec()
	assertReplay := func(want uint64) {
		t.Helper()
		capture.Sample()
		for _, value := range capture.Latest().Values {
			if value.Name == spec.Name && value.Available && value.Current == want {
				return
			}
		}
		t.Fatalf("measured send queue differs from original retained bytes, want %d", want)
	}
	assertReplay(7)
	next, err := state.PrepareReplacement(2, wire.Continuity{ReceiveNext: 2})
	if err != nil {
		t.Fatal(err)
	}
	assertReplay(7)
	next.Commit()
	assertReplay(5)
}
