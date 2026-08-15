package serviceconn

import "testing"

func TestFullSendQueueUnblocksWhenReattachmentRequiresReplay(t *testing.T) {
	stream := &recoveryStream{sendData: make([]byte, logicalQueueLimit),
		sendEnd: logicalQueueLimit, sendNext: logicalQueueLimit}
	if !stream.sendQueueBlockedLocked() {
		t.Fatal("fully transmitted unacknowledged queue did not apply backpressure")
	}
	stream.sendNext = 0
	if stream.sendQueueBlockedLocked() {
		t.Fatal("reattachment replay remained blocked behind the full send queue")
	}
}
