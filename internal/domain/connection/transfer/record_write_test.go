package transfer

import (
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

func TestAcknowledgementCompletesOnlyThroughOriginalWrite(t *testing.T) {
	state, err := New(1, 16, 16)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.AcceptRecord(wire.Record{Data: &wire.Data{AttachmentGeneration: 1, Payload: []byte("a")}}); err != nil {
		t.Fatal(err)
	}
	record, ticket, err := state.BeginRecordWrite()
	if err != nil || ticket == nil || record.Acknowledgement == nil || record.Acknowledgement.Offset != 1 {
		t.Fatal("original acknowledgement unavailable", err)
	}
	copied := *ticket
	if err := state.FinishRecordWrite(&copied, nil); !errors.Is(err, ErrIntegrity) {
		t.Fatal("copied acknowledgement completed original write", err)
	}
	if err := state.AcceptRecord(wire.Record{Data: &wire.Data{AttachmentGeneration: 1, Offset: 1, Payload: []byte("b")}}); err != nil {
		t.Fatal(err)
	}
	if err := state.FinishRecordWrite(ticket, nil); err != nil {
		t.Fatal(err)
	}
	record, pending, err := state.BeginRecordWrite()
	if err != nil || pending == nil || record.Acknowledgement == nil || record.Acknowledgement.Offset != 2 {
		t.Fatal("older physical write hid the newly queued acknowledgement", err)
	}
}
