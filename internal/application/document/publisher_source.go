package document

import (
	"errors"
	"io"
)

func (owner *publisherConnections) supplySnapshotBlock(stream *publisherStream) error {
	if owner.source == nil || stream.sourceEOF || len(stream.sourcePending) != 0 || stream.receiveCredit == 0 {
		return errors.New("publisher worker requested an unsolicited snapshot block")
	}
	remaining := owner.sourceSize - stream.sourceOffset
	if stream.failed || remaining == 0 {
		stream.sourceEOF = true
		return owner.frame(workerFrame{kind: sourceBlockFrame, id: stream.id})
	}
	size := min(remaining, workerFrameLimit, int64(stream.receiveCredit))
	block := make([]byte, size)
	n, err := owner.source.ReadAt(block, stream.sourceOffset)
	if n != len(block) || err != nil && !errors.Is(err, io.EOF) {
		clear(block)
		return errors.New("retained Publisher snapshot read failed")
	}
	stream.sourcePending = block
	stream.sourceOffset += int64(n)
	return owner.frame(workerFrame{kind: sourceBlockFrame, id: stream.id, body: block})
}
