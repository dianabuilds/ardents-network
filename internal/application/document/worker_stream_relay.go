package document

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

type relayDirection struct {
	credit   uint32
	bytes    uint64
	finished bool
	opened   bool
}

// The fixed worker moves one stream between two local attachments selected by
// Endpoint. It sees no address, pathname, command, key or remote metadata.
// Explicit credit from the receiver is propagated back to the original sender.
func relayWorkerStreams(attachment io.ReadWriter) error {
	directions := [2]relayDirection{{credit: workerCredit}, {credit: workerCredit}}
	for {
		frame, err := readWorkerFrame(attachment)
		if err != nil {
			return err
		}
		var index int
		switch frame.id {
		case 1:
			index = 0
		case 3:
			index = 1
		default:
			return errors.New("byte-stream worker attachment is invalid")
		}
		source, target := &directions[index], &directions[1-index]
		if frame.kind == 1 {
			if source.opened {
				return errors.New("byte-stream worker attachment was already opened")
			}
			source.opened = true
			continue
		}
		if !source.opened || !target.opened {
			return errors.New("byte-stream worker requires both attachments")
		}
		switch frame.kind {
		case 2:
			size := uint32(len(frame.body))
			if source.finished || size > source.credit || uint64(size) > endpointstream.ByteStreamDirectionalLimit-source.bytes {
				return errors.New("byte-stream worker input exceeds its credit")
			}
			source.credit -= size
			source.bytes += uint64(size)
		case 3:
			credit := binary.BigEndian.Uint32(frame.body)
			if credit == 0 || credit > workerCredit-target.credit {
				return errors.New("byte-stream worker credit is invalid")
			}
			target.credit += credit
		case 4:
			if source.finished {
				return errors.New("byte-stream worker input already ended")
			}
			source.finished = true
		case 5:
			if frame.body[0] != 0 || !source.finished || !target.finished {
				return errors.New("byte-stream worker exchange was interrupted")
			}
			// The host authenticates this receipt on the worker attachment.
			// Socket EOF alone carries no authenticated completion.
			return writeWorkerFrame(attachment, frame)
		}
		frame.id = 4 - frame.id
		if err := writeWorkerFrame(attachment, frame); err != nil {
			return err
		}
	}
}
