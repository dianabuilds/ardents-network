package connection

import (
	"net"
	"sync"
	"testing"
	"time"
)

// A receipt received on generation 1 cannot complete the confirmation exchange
// on generation 2. This reproduces the state captured by the lost-receipt
// journey when recovery commits before the old confirmation write completes.
func TestSettledTerminalReplaysReceiptBeforeOldConfirmationCompletes(t *testing.T) {
	for _, confirmationSent := range []bool{false, true} {
		name := "pending confirmation"
		if confirmationSent {
			name = "possibly lost confirmation"
		}
		t.Run(name, func(t *testing.T) {
			writer, reader := net.Pipe()
			defer writer.Close()
			defer reader.Close()
			if err := reader.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			attachment := terminalRecoveryAttachment(t, writer, 2, [32]byte{4}, [32]byte{5})
			stream := &Stream{current: attachment, sendBase: 8, sendNext: 8, sendEnd: 8,
				localTerminal: true, terminalSettled: true, terminalGeneration: 1,
				terminalOffset: 8, terminalAcknowledgedGeneration: 1,
				terminalConfirmationPending: true, terminalConfirmationSent: confirmationSent,
				terminalConfirmationGeneration: 1, terminalConfirmationOffset: 8}
			stream.cond = sync.NewCond(&stream.mu)
			stream.startSettledTerminalReplay()
			record, err := ReadStream(reader)
			// Unblock and join a failed writer as well, so a regression cannot leave
			// a replay goroutine after this test returns.
			_ = reader.Close()
			stream.mu.Lock()
			for stream.terminalReplaying && stream.terminal == nil {
				stream.cond.Wait()
			}
			generation := stream.terminalGeneration
			stream.mu.Unlock()
			if err != nil {
				t.Fatalf("replacement received no Terminal for receipt renewal: %v", err)
			}
			if record.Terminal == nil || record.Terminal.AttachmentGeneration != 2 || record.Terminal.Offset != 8 || generation != 2 {
				t.Fatalf("replacement Terminal = %+v, settled generation = %d", record.Terminal, generation)
			}
		})
	}
}

func TestRunBoundedRecoversLostTerminalReceiptAndConfirmation(t *testing.T) {
	runTerminalReceiptRecovery(t, true)
}
