package connection

import (
	"bytes"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

// Two independent control ledgers exchange actual canonical records. These
// protocol fixtures grant no TLS, operation, Continuity or replacement authority.
// The surrounding authenticated owner must supply those separate obligations.
func TestTerminalReplayAcrossBothRolesAndEveryControlCut(t *testing.T) {
	for _, role := range []string{"initiator", "publisher"} {
		for _, kind := range []string{"terminal", "receipt", "confirmation"} {
			for _, cut := range []string{"before-write", "written-not-delivered", "received-before-finish", "after-finish"} {
				t.Run(role+"/"+kind+"/"+cut, func(t *testing.T) {
					peers := [2]*terminalExchange{}
					offsets := [2]uint64{7, 9}
					for index := range peers {
						var err error
						peers[index], err = newTerminalExchange(1)
						if err != nil {
							t.Fatal(err)
						}
						if err := peers[index].closeLocal(offsets[index]); err != nil {
							t.Fatal(err)
						}
					}
					selected := 0
					if role == "publisher" {
						selected = 1
					}
					var eof [2]int
					var interrupted *terminalWrite
					found := false
					for round := 0; round < 6 && !found; round++ {
						for index, sender := range peers {
							ticket, err := sender.beginWrite(sender.localOffset)
							if err != nil {
								t.Fatal(err)
							}
							if ticket == nil {
								continue
							}
							if index == selected && terminalControlKind(ticket.record) == kind {
								found = true
								interrupted = ticket
								switch cut {
								case "before-write":
									failure := errors.New("original interrupted control write")
									if current, err := sender.finishWrite(ticket, failure); !current || err != failure {
										t.Fatal("failed control lost its original outcome", current, err)
									}
									interrupted = nil
								case "written-not-delivered":
									finishControl(t, sender, ticket)
									interrupted = nil
								case "received-before-finish":
									deliverTerminalControl(t, peers[1-index], ticket.record, offsets[index], &eof[1-index])
								case "after-finish":
									deliverTerminalControl(t, peers[1-index], ticket.record, offsets[index], &eof[1-index])
									finishControl(t, sender, ticket)
									interrupted = nil
								}
								break
							}
							deliverTerminalControl(t, peers[1-index], ticket.record, offsets[index], &eof[1-index])
							finishControl(t, sender, ticket)
						}
					}
					if !found {
						t.Fatal("control schedule never reached its selected causal boundary")
					}
					for index, peer := range peers {
						if err := peer.replace(2); err != nil || peer.complete(offsets[index]) {
							t.Fatal("old control proofs completed the replacement", err)
						}
						for _, confirmation := range []bool{false, true} {
							if err := peer.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: 1, Offset: offsets[index], Terminal: true, TerminalConfirmation: confirmation}); !errors.Is(err, errIntegrity) {
								t.Fatal("stale receipt or confirmation crossed the replacement", err)
							}
						}
					}
					if interrupted != nil {
						if current, err := peers[selected].finishWrite(interrupted, nil); current || err != nil {
							t.Fatal("late old write changed the current generation", current, err)
						}
					}
					for round := 0; round < 6 && (!peers[0].complete(offsets[0]) || !peers[1].complete(offsets[1])); round++ {
						for index, sender := range peers {
							ticket, err := sender.beginWrite(sender.localOffset)
							if err != nil {
								t.Fatal(err)
							}
							if ticket != nil {
								deliverTerminalControl(t, peers[1-index], ticket.record, offsets[index], &eof[1-index])
								finishControl(t, sender, ticket)
							}
						}
					}
					if !peers[0].complete(offsets[0]) || !peers[1].complete(offsets[1]) || eof != [2]int{1, 1} {
						t.Fatal("replacement failed both proofs or delivered another EOF", eof)
					}
					// All current causal prerequisites now hold. Refusal must
					// depend on the old generation, not an absent prerequisite.
					for _, peer := range peers {
						for _, stale := range []wire.Acknowledgement{
							{AttachmentGeneration: 1, Offset: peer.localOffset, Terminal: true},
							{AttachmentGeneration: 1, Offset: peer.remoteOffset, Terminal: true, TerminalConfirmation: true},
						} {
							if err := peer.receiveAcknowledgement(stale); !errors.Is(err, errIntegrity) {
								t.Fatal("old generation reused current causal prerequisites", err)
							}
						}
					}
				})
			}
		}
	}
}

func terminalControlKind(record wire.StreamRecord) string {
	if record.Terminal != nil {
		return "terminal"
	}
	if record.Acknowledgement.TerminalConfirmation {
		return "confirmation"
	}
	return "receipt"
}

func deliverTerminalControl(t *testing.T, receiver *terminalExchange, record wire.StreamRecord, receiveNext uint64, eof *int) {
	t.Helper()
	var encoded bytes.Buffer
	if err := wire.Write(&encoded, wire.Record{Terminal: record.Terminal, Acknowledgement: record.Acknowledgement}); err != nil {
		t.Fatal(err)
	}
	decoded, err := wire.Read(&encoded)
	if err != nil || encoded.Len() != 0 {
		t.Fatal("canonical control did not decode independently", err)
	}
	if decoded.Terminal != nil {
		first, err := receiver.receiveTerminal(*decoded.Terminal, receiveNext, false)
		if err != nil {
			t.Fatal(err)
		}
		if first {
			*eof++
		}
	} else if err := receiver.receiveAcknowledgement(*decoded.Acknowledgement); err != nil {
		t.Fatal(err)
	}
}
