package connection

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	"math"
	"testing"
)

func preparedTerminals(t *testing.T) *terminalExchange {
	t.Helper()
	x, err := newTerminalExchange(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := x.closeLocal(7); err != nil {
		t.Fatal(err)
	}
	first, err := x.receiveTerminal(wire.Terminal{AttachmentGeneration: 1, Offset: 9}, 9, false)
	if err != nil || !first {
		t.Fatalf("first Terminal=%t %v", first, err)
	}
	return x
}

func finishControl(t *testing.T, x *terminalExchange, ticket *terminalWrite) {
	t.Helper()
	if ticket == nil {
		t.Fatal("missing control")
	}
	if current, err := x.finishWrite(ticket, nil); !current || err != nil {
		t.Fatalf("finish current=%t error=%v", current, err)
	}
}

func completeTerminalExchange(t *testing.T, x *terminalExchange) {
	t.Helper()
	terminal, err := x.beginWrite(7)
	if err != nil || terminal == nil || terminal.record.Terminal == nil || terminal.record.Terminal.Offset != 7 {
		t.Fatal("Terminal must be first")
	}
	finishControl(t, x, terminal)
	receipt, err := x.beginWrite(7)
	if err != nil || receipt == nil || receipt.record.Acknowledgement == nil || receipt.record.Acknowledgement.TerminalConfirmation || receipt.record.Acknowledgement.Offset != 9 {
		t.Fatal("receipt must follow peer Terminal")
	}
	// A peer can observe the complete physical write before this writer returns.
	if err := x.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: x.generation, Offset: 9, Terminal: true, TerminalConfirmation: true}); err != nil {
		t.Fatal(err)
	}
	finishControl(t, x, receipt)
	if err := x.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: x.generation, Offset: 7, Terminal: true, TerminalConfirmation: false}); err != nil {
		t.Fatal(err)
	}
	confirmation, err := x.beginWrite(7)
	if err != nil || confirmation == nil || confirmation.record.Acknowledgement == nil || !confirmation.record.Acknowledgement.TerminalConfirmation || confirmation.record.Acknowledgement.Offset != 7 {
		t.Fatal("confirmation must match local receipt")
	}
	if x.complete(7) {
		t.Fatal("in-progress confirmation became success")
	}
	finishControl(t, x, confirmation)
	if !x.complete(7) {
		t.Fatal("complete authenticated control did not settle")
	}
}

func TestTerminalControlRequiresEveryCausalProof(t *testing.T) {
	t.Parallel()
	x := preparedTerminals(t)
	if x.complete(7) {
		t.Fatal("EOF alone became success")
	}
	if err := x.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: 1, Offset: 7, Terminal: true, TerminalConfirmation: false}); !errors.Is(err, errIntegrity) {
		t.Fatal("receipt before Terminal accepted")
	}
	if err := x.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: 1, Offset: 9, Terminal: true, TerminalConfirmation: true}); !errors.Is(err, errIntegrity) {
		t.Fatal("confirmation before receipt accepted")
	}
	if ticket, err := x.beginWrite(6); err != nil || ticket != nil {
		t.Fatal("Terminal overtook pending Data")
	}
	completeTerminalExchange(t, x)
	if x.complete(6) {
		t.Fatal("unacknowledged bytes became success")
	}
	if err := x.closeLocal(7); err != nil {
		t.Fatal("repeated CloseInput refused")
	}
	if err := x.closeLocal(8); !errors.Is(err, errIntegrity) {
		t.Fatal("changed local final offset accepted")
	}
	first, err := x.receiveTerminal(wire.Terminal{AttachmentGeneration: 1, Offset: 9}, 9, false)
	if err != nil || first {
		t.Fatal("matching repeated Terminal presented second EOF")
	}
}

func TestTerminalReplacementRenewsControlWithoutAnotherEOF(t *testing.T) {
	t.Parallel()
	x := preparedTerminals(t)
	completeTerminalExchange(t, x)
	if err := x.replace(2); err != nil {
		t.Fatal(err)
	}
	if x.complete(7) {
		t.Fatal("old confirmation settled replacement")
	}
	for _, value := range []wire.Acknowledgement{
		{AttachmentGeneration: 1, Offset: 7, Terminal: true},
		{AttachmentGeneration: 1, Offset: 9, Terminal: true, TerminalConfirmation: true},
		{AttachmentGeneration: 3, Offset: 7, Terminal: true},
		{AttachmentGeneration: 2, Offset: 8, Terminal: true},
		{AttachmentGeneration: 2, Offset: 9, Terminal: true, TerminalConfirmation: true},
	} {
		if err := x.receiveAcknowledgement(value); !errors.Is(err, errIntegrity) {
			t.Fatalf("foreign/pre-causal control accepted: %+v", value)
		}
	}
	terminal, err := x.beginWrite(7)
	if err != nil || terminal == nil || terminal.record.Terminal == nil {
		t.Fatal("replacement failed to replay local Terminal")
	}
	finishControl(t, x, terminal)
	if ticket, err := x.beginWrite(7); err != nil || ticket != nil {
		t.Fatal("receipt emitted before current peer Terminal")
	}
	first, err := x.receiveTerminal(wire.Terminal{AttachmentGeneration: 2, Offset: 9}, 9, false)
	if err != nil || first {
		t.Fatal("replacement produced another EOF")
	}
	receipt, _ := x.beginWrite(7)
	finishControl(t, x, receipt)
	if err := x.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: 2, Offset: 9, Terminal: true, TerminalConfirmation: true}); err != nil {
		t.Fatal(err)
	}
	if err := x.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: 2, Offset: 7, Terminal: true, TerminalConfirmation: false}); err != nil {
		t.Fatal(err)
	}
	confirmation, _ := x.beginWrite(7)
	finishControl(t, x, confirmation)
	if !x.complete(7) {
		t.Fatal("replacement control did not settle")
	}
}

func TestTerminalStaleWriteCannotCompleteReplacement(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"terminal", "receipt", "confirmation"} {
		t.Run(boundary, func(t *testing.T) {
			x := preparedTerminals(t)
			ticket, _ := x.beginWrite(7)
			if boundary != "terminal" {
				finishControl(t, x, ticket)
				ticket, _ = x.beginWrite(7)
			}
			if boundary == "confirmation" {
				finishControl(t, x, ticket)
				if err := x.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: 1, Offset: 7, Terminal: true, TerminalConfirmation: false}); err != nil {
					t.Fatal(err)
				}
				ticket, _ = x.beginWrite(7)
			}
			if err := x.replace(2); err != nil {
				t.Fatal(err)
			}
			physical := errors.New("original physical cleanup failed")
			current, err := x.finishWrite(ticket, physical)
			if current || err != physical || x.terminalWritten || x.receiptWritten || x.confirmationWritten {
				t.Fatal("stale completion changed successor or lost original error")
			}
			if _, err := x.finishWrite(ticket, nil); !errors.Is(err, errIntegrity) {
				t.Fatal("one-use write ticket replayed")
			}
		})
	}
}

func TestTerminalReplacementRefusesCopiedAndForeignWriteTickets(t *testing.T) {
	for _, boundary := range []string{"terminal", "receipt", "confirmation"} {
		t.Run(boundary, func(t *testing.T) {
			x := preparedTerminals(t)
			ticket, _ := x.beginWrite(7)
			if boundary != "terminal" {
				finishControl(t, x, ticket)
				ticket, _ = x.beginWrite(7)
			}
			if boundary == "confirmation" {
				finishControl(t, x, ticket)
				if err := x.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: 1, Offset: 7, Terminal: true}); err != nil {
					t.Fatal(err)
				}
				ticket, _ = x.beginWrite(7)
			}
			if ticket == nil {
				t.Fatal("missing original physical writer")
			}
			copied := *ticket
			foreign := preparedTerminals(t)
			foreignTicket, _ := foreign.beginWrite(7)
			if err := x.replace(2); err != nil {
				t.Fatal(err)
			}
			for _, rejected := range []*terminalWrite{&copied, foreignTicket} {
				if current, err := x.finishWrite(rejected, nil); current || !errors.Is(err, errIntegrity) {
					t.Error("copied or foreign old writer bypassed original identity", current, err)
				}
			}
			if ticket.consumed || foreignTicket.consumed || x.active != nil || x.complete(7) {
				t.Fatal("rejected writer changed original or replacement")
			}
			physical := errors.New("retained original writer failure")
			if current, err := x.finishWrite(ticket, physical); current || err != physical || !ticket.consumed {
				t.Fatal("actual old writer lost its physical result", current, err)
			}
			finishControl(t, foreign, foreignTicket)
		})
	}
}

func TestTerminalRefusesForeignRangeGenerationAndGap(t *testing.T) {
	t.Parallel()
	x := preparedTerminals(t)
	for _, tc := range []struct {
		terminal wire.Terminal
		next     uint64
		pending  bool
	}{
		{wire.Terminal{AttachmentGeneration: 0, Offset: 9}, 9, false}, {wire.Terminal{AttachmentGeneration: 2, Offset: 9}, 9, false}, {wire.Terminal{AttachmentGeneration: 1, Offset: 10}, 9, false}, {wire.Terminal{AttachmentGeneration: 1, Offset: 9}, 9, true},
		{wire.Terminal{AttachmentGeneration: 1, Offset: 10}, 10, false},
	} {
		if _, err := x.receiveTerminal(tc.terminal, tc.next, tc.pending); !errors.Is(err, errIntegrity) {
			t.Fatal("invalid Terminal accepted")
		}
	}
	if err := x.replace(1); !errors.Is(err, errIntegrity) {
		t.Fatal("generation reused")
	}
	if err := x.replace(3); !errors.Is(err, errIntegrity) {
		t.Fatal("generation skipped")
	}
	x.generation = math.MaxUint64
	if err := x.replace(0); !errors.Is(err, errIntegrity) {
		t.Fatal("generation overflow accepted")
	}
}

func TestTerminalSingleWriterAndPhysicalFailure(t *testing.T) {
	t.Parallel()
	x := preparedTerminals(t)
	ticket, _ := x.beginWrite(7)
	if _, err := x.beginWrite(7); err == nil {
		t.Fatal("second writer admitted")
	}
	foreign := *ticket
	if _, err := x.finishWrite(&foreign, nil); !errors.Is(err, errIntegrity) {
		t.Fatal("copied write ticket accepted")
	}
	physical := errors.New("physical write failed")
	if current, err := x.finishWrite(ticket, physical); !current || err != physical || x.terminalWritten {
		t.Fatal("failed write became proof")
	}
	if err := x.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: 1, Offset: 7, Terminal: true, TerminalConfirmation: false}); !errors.Is(err, errIntegrity) {
		t.Fatal("failed Terminal admitted receipt")
	}
	completeTerminalExchange(t, x)
}

func TestTerminalMissingPeerConfirmationNeverSettles(t *testing.T) {
	x := preparedTerminals(t)
	terminal, _ := x.beginWrite(7)
	finishControl(t, x, terminal)
	receipt, _ := x.beginWrite(7)
	finishControl(t, x, receipt)
	if err := x.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: 1, Offset: 7, Terminal: true, TerminalConfirmation: false}); err != nil {
		t.Fatal(err)
	}
	confirmation, _ := x.beginWrite(7)
	finishControl(t, x, confirmation)
	if x.complete(7) {
		t.Fatal("successful local confirmation write replaced missing peer confirmation")
	}
	if err := x.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: 1, Offset: 9, Terminal: true, TerminalConfirmation: true}); err != nil {
		t.Fatal(err)
	}
	if !x.complete(7) {
		t.Fatal("final peer confirmation did not settle")
	}
}
