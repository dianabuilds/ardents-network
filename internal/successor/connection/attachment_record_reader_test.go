package connection

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

type recordInputTransport struct {
	*bytes.Reader
	reads, failAt int
	failure       error
}

func (r *recordInputTransport) Read(body []byte) (int, error) {
	r.reads++
	n, err := r.Reader.Read(body)
	if r.reads == r.failAt {
		err = errors.Join(err, r.failure)
	}
	return n, err
}

// These fixed grammar records confer no Connection or owner authority.
func recordInputFixture(t *testing.T, size int) ([]byte, []byte) {
	t.Helper()
	var data, terminal bytes.Buffer
	if err := wire.Write(&data, wire.Record{Data: &wire.Data{AttachmentGeneration: 1, Payload: bytes.Repeat([]byte{42}, size)}}); err != nil {
		t.Fatal(err)
	}
	if err := wire.Write(&terminal, wire.Record{Terminal: &wire.Terminal{AttachmentGeneration: 1, Offset: uint64(size)}}); err != nil {
		t.Fatal(err)
	}
	return data.Bytes(), terminal.Bytes()
}

func TestAttachmentRecordReaderRetainsSeparateRecordsAndPartialTail(t *testing.T) {
	data, terminal := recordInputFixture(t, 3)
	for _, partial := range []bool{false, true} {
		tail := terminal
		if partial {
			tail = terminal[:15]
		}
		transport := &recordInputTransport{Reader: bytes.NewReader(append(append([]byte(nil), data...), tail...))}
		input := &attachmentRecordReader{reader: transport}
		got, err, received := input.readRecord()
		if err != nil || got.Data == nil || received != len(data) || transport.reads != 1 {
			t.Fatal("buffered first record differs", err, received, transport.reads)
		}
		got, err, received = input.readRecord()
		if partial {
			if !errors.Is(err, io.ErrUnexpectedEOF) || received != 15 || terminalTruncation(err, received) {
				t.Fatal("partial buffered successor became an empty terminal tail", err, received)
			}
		} else if err != nil || got.Terminal == nil || received != len(terminal) || transport.reads != 1 {
			t.Fatal("buffered successor lost its own boundary", err, received, transport.reads)
		}
		input.discard()
		if input.buffer != [512]byte{} || input.reader != nil || input.start != 0 || input.end != 0 {
			t.Fatal("retired reader retained prefetched bytes or transport")
		}
	}
}

func TestAttachmentRecordReaderRejectsBytesAccompaniedByOriginalFailure(t *testing.T) {
	for _, size := range []int{3, 2048} {
		data, _ := recordInputFixture(t, size)
		cause := errors.New("original transport refused while returning record bytes")
		failAt := 1
		if size > 512 {
			failAt = 2 // Direct body read after the fixed framing scratch.
		}
		transport := &recordInputTransport{Reader: bytes.NewReader(data), failAt: failAt, failure: cause}
		input := &attachmentRecordReader{reader: transport}
		got, err, received := input.readRecord()
		if !errors.Is(err, cause) || got.Data != nil || received == 0 {
			t.Fatal("buffered or direct read hid original failure", size, err, received)
		}
		calls := transport.reads
		if _, err, _ := input.readRecord(); !errors.Is(err, cause) || transport.reads != calls {
			t.Fatal("failed reader resumed physical input", size, err)
		}
		input.discard()
	}
}
