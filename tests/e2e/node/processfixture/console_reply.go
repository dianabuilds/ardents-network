package processfixture

import (
	"bufio"
	"encoding/json"
	"errors"
)

const maxConsoleReplyBytes = 64 << 10

var errConsoleReplyTooLong = errors.New("console fixture reply exceeds its finite bound")

// ReadConsoleReply consumes the entire line, including the newline that lets a
// pipe-backed console writer finish before the fixture sends another command.
// An oversized record is terminal for the caller's console exchange.
func ReadConsoleReply(reader *bufio.Reader, reply any) error {
	var record []byte
	for {
		part, err := reader.ReadSlice('\n')
		if len(part) > maxConsoleReplyBytes-len(record) {
			return errConsoleReplyTooLong
		}
		record = append(record, part...)
		switch err {
		case nil:
			return json.Unmarshal(record, reply)
		case bufio.ErrBufferFull:
			continue
		default:
			return err
		}
	}
}
