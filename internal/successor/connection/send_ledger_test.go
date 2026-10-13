package connection

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

func TestSendRecoveryImmediatelyReplaysRetainedSuffix(t *testing.T) {
	ledger, err := newSendLedger(1, 9)
	if err != nil {
		t.Fatal(err)
	}
	input := []byte("request")
	if err := ledger.accept(input); err != nil {
		t.Fatal(err)
	}
	input[0] = 'X'
	first, err := ledger.beginWrite()
	if err != nil || string(first.record.Payload) != "request" {
		t.Fatal("accepted bytes were not owned")
	}
	if err := ledger.acknowledge(1, 3); err != nil {
		t.Fatal(err)
	}
	if err := ledger.replace(2, 3); err != nil {
		t.Fatal(err)
	}
	physical := errors.New("old writer failed")
	if current, err := ledger.finishWrite(first, physical); current || err != physical || ledger.next != 3 {
		t.Fatal("stale writer changed replacement or erased error")
	}
	// No new Application input/EOF triggers this replay.
	replay, err := ledger.beginWrite()
	if err != nil || replay.record.Offset != 3 || replay.record.AttachmentGeneration != 2 || string(replay.record.Payload) != "uest" {
		t.Fatal("immediate ordered suffix replay differs")
	}
	if current, err := ledger.finishWrite(replay, nil); !current || err != nil {
		t.Fatal(err)
	}
	if err := ledger.accept([]byte("ok")); err != nil {
		t.Fatal(err)
	}
	next, err := ledger.beginWrite()
	if err != nil || next.record.Offset != 7 || string(next.record.Payload) != "ok" {
		t.Fatal("later bytes overtook replay")
	}
	if err := ledger.accept([]byte("!")); !errors.Is(err, errIntegrity) {
		t.Fatal("directional counter reset on replacement")
	}
	end := ledger.closeInput()
	if end != 9 || ledger.closeInput() != end {
		t.Fatal("repeat CloseInput changed final offset")
	}
	if err := ledger.accept([]byte("later")); !errors.Is(err, errIntegrity) {
		t.Fatal("bytes admitted after CloseInput")
	}
}

func TestSendQueueBoundAppliesBackpressureButAllowsReplay(t *testing.T) {
	ledger, _ := newSendLedger(1, 1<<20)
	payload := bytes.Repeat([]byte{42}, 16384)
	for range 16 {
		if err := ledger.accept(payload); err != nil {
			t.Fatal(err)
		}
		ticket, err := ledger.beginWrite()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ledger.finishWrite(ticket, nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(ledger.data) != 262144 {
		t.Fatal("queue did not retain exact bound")
	}
	if err := ledger.accept([]byte{1}); !errors.Is(err, errBackpressure) {
		t.Fatal("queue bound bypassed")
	}
	if err := ledger.replace(2, 0); err != nil {
		t.Fatal(err)
	}
	if replay, err := ledger.beginWrite(); err != nil || replay == nil || replay.record.Offset != 0 || !bytes.Equal(replay.record.Payload, payload) {
		t.Fatal("full queue blocked required replay")
	}
	if err := ledger.acknowledge(2, 16384); err != nil {
		t.Fatal(err)
	}
	if err := ledger.accept(payload); err != nil {
		t.Fatal("ACK failed to release bounded space")
	}
	if len(ledger.data) != 262144 || ledger.end != 278528 {
		t.Fatal("replacement reset byte counter or expanded queue")
	}
}

func TestSendAcknowledgementRequiresExposedCurrentRange(t *testing.T) {
	ledger, _ := newSendLedger(1, 20)
	if err := ledger.accept([]byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := ledger.acknowledge(1, 5); !errors.Is(err, errIntegrity) {
		t.Fatal("unsent queued range acknowledged")
	}
	ticket, _ := ledger.beginWrite()
	if err := ledger.acknowledge(1, 5); err != nil {
		t.Fatal("peer-observed in-flight range refused")
	}
	if _, err := ledger.finishWrite(ticket, nil); err != nil || ledger.next != 5 {
		t.Fatal("ACK/write race rolled offsets backward")
	}
	if err := ledger.acknowledge(1, 2); err != nil || ledger.base != 5 {
		t.Fatal("matching delayed ACK changed base")
	}
	for _, value := range [][2]uint64{{0, 5}, {2, 5}, {1, 6}, {1, math.MaxUint64}} {
		if err := ledger.acknowledge(value[0], value[1]); !errors.Is(err, errIntegrity) {
			t.Fatal("foreign/future ACK accepted")
		}
	}
	if err := ledger.replace(2, 4); !errors.Is(err, errIntegrity) {
		t.Fatal("Continuity ACK rollback accepted")
	}
	if err := ledger.replace(2, 5); err != nil {
		t.Fatal(err)
	}
}

func TestSendWriterTicketsAndOffsetOverflow(t *testing.T) {
	ledger, _ := newSendLedger(1, 20)
	if err := ledger.accept([]byte("bytes")); err != nil {
		t.Fatal(err)
	}
	ticket, _ := ledger.beginWrite()
	if _, err := ledger.beginWrite(); err == nil {
		t.Fatal("second physical writer admitted")
	}
	copyTicket := *ticket
	if _, err := ledger.finishWrite(&copyTicket, nil); !errors.Is(err, errIntegrity) {
		t.Fatal("copied ticket accepted")
	}
	physical := errors.New("output failed")
	if current, err := ledger.finishWrite(ticket, physical); !current || err != physical || ledger.next != 0 {
		t.Fatal("failed output advanced offsets")
	}
	if _, err := ledger.finishWrite(ticket, nil); !errors.Is(err, errIntegrity) {
		t.Fatal("write ticket consumed twice")
	}
	ledger.end, ledger.limit = math.MaxUint64, math.MaxUint64
	if err := ledger.accept([]byte{1}); !errors.Is(err, errIntegrity) {
		t.Fatal("offset overflow accepted")
	}
	ledger.generation = math.MaxUint64
	if err := ledger.replace(0, 0); !errors.Is(err, errIntegrity) {
		t.Fatal("Attachment generation overflow accepted")
	}
}

func TestSendReplacementRefusesCopiedAndForeignWriteTickets(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		name := "original"
		if replacement {
			name = "replacement"
		}
		t.Run(name, func(t *testing.T) {
			ledger, _ := newSendLedger(1, 20)
			foreign, _ := newSendLedger(1, 20)
			for _, owner := range []*sendLedger{ledger, foreign} {
				if err := owner.accept([]byte("bytes")); err != nil {
					t.Fatal(err)
				}
			}
			ticket, _ := ledger.beginWrite()
			foreignTicket, _ := foreign.beginWrite()
			copied := *ticket
			if replacement {
				if err := ledger.replace(2, 0); err != nil {
					t.Fatal(err)
				}
			}
			for _, rejected := range []*dataWrite{&copied, foreignTicket} {
				if current, err := ledger.finishWrite(rejected, nil); current || !errors.Is(err, errIntegrity) {
					t.Error("copied or foreign Data writer bypassed original identity", current, err)
				}
			}
			if ticket.consumed || foreignTicket.consumed || ledger.next != 0 || ledger.base != 0 || ledger.end != 5 {
				t.Fatal("rejected writer changed originals or byte counters")
			}
			physical := errors.New("original Data output failed")
			if current, err := ledger.finishWrite(ticket, physical); current == replacement || err != physical || !ticket.consumed {
				t.Fatal("actual Data writer lost its retained result", current, err)
			}
			if current, err := foreign.finishWrite(foreignTicket, nil); !current || err != nil || foreign.next != 5 {
				t.Fatal("rejected foreign ticket lost original completion", current, err)
			}
			if replacement {
				replay, err := ledger.beginWrite()
				if err != nil || replay.record.AttachmentGeneration != 2 || replay.record.Offset != 0 || string(replay.record.Payload) != "bytes" {
					t.Fatal("rejected writer damaged immediate replay", err)
				}
			}
		})
	}
}

func TestSendReplacementCannotAcknowledgeUnreplayedOldExposure(t *testing.T) {
	ledger, _ := newSendLedger(1, 30000)
	for _, size := range []int{16384, 3616} {
		if err := ledger.accept(bytes.Repeat([]byte{42}, size)); err != nil {
			t.Fatal(err)
		}
		ticket, err := ledger.beginWrite()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ledger.finishWrite(ticket, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := ledger.replace(2, 100); err != nil {
		t.Fatal(err)
	}
	if err := ledger.acknowledge(2, 20000); !errors.Is(err, errIntegrity) {
		t.Fatal("new Attachment acknowledged old exposure before replay")
	}
	if ledger.base != 100 || ledger.next != 100 || ledger.end != 20000 || len(ledger.data) != 19900 {
		t.Fatal("refused ACK discarded original retained suffix or counters")
	}
	if err := ledger.acknowledge(2, 100); err != nil {
		t.Fatal("exact Continuity-committed ACK refused")
	}
	replay, err := ledger.beginWrite()
	if err != nil || replay.record.Offset != 100 || len(replay.record.Payload) != 16384 {
		t.Fatal("replacement did not immediately expose first ordered suffix")
	}
	if err := ledger.acknowledge(2, 20000); !errors.Is(err, errIntegrity) {
		t.Fatal("new Attachment acknowledged beyond its current replay writer")
	}
	if err := ledger.acknowledge(2, 16484); err != nil {
		t.Fatal("actual in-flight replay ACK refused")
	}
	if _, err := ledger.finishWrite(replay, nil); err != nil {
		t.Fatal(err)
	}
	if ledger.base != 16484 || ledger.end != 20000 || len(ledger.data) != 3516 {
		t.Fatal("current replay ACK changed original total or lost remaining suffix")
	}
}
