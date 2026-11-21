package connection

import (
	"io"
)

// Record parsing must observe no additional plaintext for a TLS-tail
// truncation exception. A partially received Service record is not an empty
// TLS tail, even when its final physical error is EOF.
type observedRecordRead struct {
	reader   io.Reader
	received int
}

// Application settlement survives replacement, but the new Attachment must
// renew every Terminal proof before its clean Route close can retire the tail.
// The caller separately checks live authority and that exact child's witness.
func (stream *Stream) terminalTailCompleteLocked() bool {
	return stream.settled && stream.terminal.complete(stream.send.base)
}

func (read *observedRecordRead) Read(body []byte) (int, error) {
	n, err := read.reader.Read(body)
	read.received += n
	return n, err
}

// A clean Route terminal permits only TLS truncation after the authenticated
// exchange is complete. Joined authority, integrity and cleanup errors cannot
// be hidden merely because one of their causes is EOF.
func terminalTruncation(err error, received int) bool {
	if received != 0 {
		return false
	}
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !terminalTruncation(cause, 0) {
				return false
			}
		}
		return true
	}
	return false
}

func (attachment attachmentIO) decodedPeerClose() error {
	if attachment.owner == nil || attachment.owner.self != attachment.owner {
		return errIntegrity
	}
	if attachment.initiator != nil {
		physical := attachment.initiator.physical
		proof, err := physical.DecodedPeerClose()
		if err != nil {
			return err
		}
		return proof.CheckOriginal(physical)
	}
	if attachment.service != nil {
		_, err := attachment.service.DecodedPeerClose()
		return err
	}
	return errIntegrity
}
