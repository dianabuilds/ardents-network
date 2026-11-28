package document

import (
	"bytes"
	"testing"
)

func TestPublisherSnapshotSourceRejectsUnsolicitedRequestsAndOutput(t *testing.T) {
	stream := &publisherStream{id: 1, receiveCredit: workerCredit, sourcePending: []byte{0xff}, queue: make([]byte, workerCredit)}
	owner := &publisherConnections{
		source: bytes.NewReader([]byte{0xff, 1}), sourceSize: 2,
		byID: map[uint32]*publisherStream{1: stream},
	}
	// A worker must return the block it already has before asking for more.
	if err := owner.fromWorker(workerFrame{kind: sourceRequestFrame, id: 1}); err == nil {
		t.Fatal("worker obtained another snapshot block before returning the first")
	}
	if err := owner.fromWorker(workerFrame{kind: 2, id: 1, body: []byte{0x80}}); err == nil {
		t.Fatal("worker replaced bytes from the selected snapshot")
	}
	if err := owner.fromWorker(workerFrame{kind: 4, id: 1}); err == nil {
		t.Fatal("worker ended a snapshot stream before the retained source")
	}
}
