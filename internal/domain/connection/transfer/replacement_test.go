package transfer

import (
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

func TestRejectedReplacementKeepsBothDirectionsAndGeneration(t *testing.T) {
	state, err := New(1, 64, 64)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Write([]byte("request")); err != nil {
		t.Fatal(err)
	}
	record, ticket, err := state.BeginRecordWrite()
	if err != nil || ticket == nil || record.Data == nil {
		t.Fatal("missing original replayable write", err)
	}
	if err := state.FinishRecordWrite(ticket, nil); err != nil {
		t.Fatal(err)
	}
	if err := state.AcceptRecord(wire.Record{Data: &wire.Data{AttachmentGeneration: 1, Payload: []byte("reply")}}); err != nil {
		t.Fatal(err)
	}

	before := state.Progress()
	if candidate, err := state.PrepareReplacement(2, wire.Continuity{ReceiveNext: 2, SendBase: 6, SendEnd: 6}); candidate != nil || !errors.Is(err, ErrIntegrity) {
		t.Fatal("inconsistent peer receive range allowed partial replacement", err)
	}
	candidate, err := state.PrepareReplacement(2, wire.Continuity{ReceiveNext: 2, SendEnd: 5})
	if err != nil {
		t.Fatal(err)
	}
	candidate.Discard()
	if state.Generation() != 1 || state.Progress() != before {
		t.Fatal("a rejected candidate changed the original generation or ranges")
	}

	candidate, err = state.PrepareReplacement(2, wire.Continuity{ReceiveNext: 2, SendEnd: 5})
	if err != nil {
		t.Fatal(err)
	}
	candidate.Commit()
	replay, pending, err := state.BeginRecordWrite()
	if err != nil || pending == nil || replay.Data == nil || replay.Data.AttachmentGeneration != 2 ||
		replay.Data.Offset != 2 || string(replay.Data.Payload) != "quest" {
		t.Fatal("replacement lost the unchanged original replay suffix", err, replay)
	}
	unread := make([]byte, 5)
	if n, err := state.Read(unread); n != 5 || err != nil || string(unread) != "reply" {
		t.Fatal("replacement lost unread original bytes", n, err, string(unread))
	}
}
