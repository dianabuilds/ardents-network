//go:build linux

package textdocument

import "encoding/binary"

// nextFrame schedules one available frame per stream in round-robin order.
// Snapshot/result slices are shared immutable data, never copied per Connection.
func (owner *workerMultiplexer) nextFrame() (workerFrame, bool) {
	if owner.readerResult && !owner.resultAnnounced {
		if result := owner.streams[2]; result != nil {
			return workerFrame{kind: 6, id: 2, body: result.header, current: result}, true
		}
	}
	for offset := range len(owner.order) {
		stream := owner.streams[owner.order[(owner.cursor+offset)%len(owner.order)]]
		if stream.rejected {
			if !stream.closeSent {
				return workerFrame{kind: 5, id: stream.id, body: []byte{1}, current: stream}, true
			}
			continue
		}
		if stream.id%2 == 1 && stream.receiveCredit < workerCredit && !stream.receivedEOF {
			body := make([]byte, 4)
			binary.BigEndian.PutUint32(body, workerCredit-stream.receiveCredit)
			return workerFrame{kind: 3, id: stream.id, body: body, current: stream}, true
		}
		if !stream.ready || stream.sentEOF {
			continue
		}
		if stream.offset == len(stream.header)+len(stream.output) {
			return workerFrame{kind: 4, id: stream.id, current: stream}, true
		}
		if stream.sendCredit == 0 {
			continue
		}
		var body []byte
		if stream.offset < len(stream.header) {
			body = stream.header[stream.offset:]
		} else {
			body = stream.output[stream.offset-len(stream.header):]
		}
		length := min(len(body), workerFrameLimit, int(stream.sendCredit))
		return workerFrame{kind: 2, id: stream.id, body: body[:length], current: stream}, true
	}
	return workerFrame{}, false
}

func (owner *workerMultiplexer) sent(frame workerFrame) {
	stream := owner.streams[frame.id]
	switch frame.kind {
	case 2:
		stream.offset += len(frame.body)
		stream.sendCredit -= uint32(len(frame.body))
	case 3:
		stream.receiveCredit += binary.BigEndian.Uint32(frame.body)
	case 4:
		stream.sentEOF = true
	case 5:
		stream.closeSent = true
	case 6:
		owner.resultAnnounced = true
		stream.header = nil
	}
	for index, id := range owner.order {
		if id == frame.id {
			owner.cursor = index + 1
			return
		}
	}
}
