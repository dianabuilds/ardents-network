package wire

import (
	"bytes"
	"errors"
	"testing"
)

type recordHeaderTransport struct {
	*bytes.Reader
	reads, chunk int
	failure      error
}

func (reader *recordHeaderTransport) Read(body []byte) (int, error) {
	reader.reads++
	if reader.chunk > 0 && len(body) > reader.chunk {
		body = body[:reader.chunk]
	}
	n, err := reader.Reader.Read(body)
	return n, errors.Join(err, reader.failure)
}

func TestRecordHeaderCoalescesWithoutReadingBodyOrFollowingRecordAhead(t *testing.T) {
	first := independentEnvelope(4, append(independentOffsets(1, 0), 0, 1, 42))
	second := independentEnvelope(6, independentOffsets(1, 1))
	for _, chunk := range []int{0, 7, 31} {
		reader := &recordHeaderTransport{Reader: bytes.NewReader(append(append([]byte(nil), first...), second...)), chunk: chunk}
		got, err := ReadStream(reader)
		if err != nil || got.Data == nil || len(got.Data.Payload) != 1 || got.Data.Payload[0] != 42 {
			t.Fatal("coalesced or fragmented header changed the record", chunk, err)
		}
		if chunk == 0 && reader.reads != 2 {
			t.Fatal("header repeated a transport observation", reader.reads)
		}
		if reader.Len() != len(second) {
			t.Fatal("record read consumed its successor", chunk, reader.Len())
		}
		got, err = ReadStream(reader)
		if err != nil || got.Terminal == nil || got.Terminal.Offset != 1 || reader.Len() != 0 {
			t.Fatal("following record was not preserved", chunk, err)
		}
	}
}

func TestRecordHeaderRetainsFailureWithCompleteHeaderBeforeBodyRead(t *testing.T) {
	frame := independentEnvelope(4, append(independentOffsets(1, 0), 0, 1, 42))
	cause := errors.New("original header read failed with bytes")
	reader := &recordHeaderTransport{Reader: bytes.NewReader(frame), failure: cause}
	if record, err := ReadStream(reader); !errors.Is(err, cause) || record.Data != nil {
		t.Fatal("complete buffered header hid its original failure", err)
	}
	if reader.reads != 1 || reader.Len() != len(frame)-32 {
		t.Fatal("failed header acquired body bytes", reader.reads, reader.Len())
	}
}
