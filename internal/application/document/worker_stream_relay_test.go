package document

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func TestByteStreamWorkerRelaysBothDirectionsAndHalfClose(t *testing.T) {
	var input, output bytes.Buffer
	credit := binary.BigEndian.AppendUint32(nil, 2)
	frames := []workerFrame{
		{kind: 1, id: 1}, {kind: 1, id: 3},
		{kind: 2, id: 1, body: []byte{0, 255}}, {kind: 3, id: 3, body: credit},
		{kind: 4, id: 1}, {kind: 2, id: 3, body: []byte{129, 0}},
		{kind: 4, id: 3}, {kind: 5, id: 1, body: []byte{0}},
	}
	for _, frame := range frames {
		if err := writeWorkerFrame(&input, frame); err != nil {
			t.Fatal(err)
		}
	}
	if err := relayWorkerStreams(struct {
		io.Reader
		io.Writer
	}{&input, &output}); err != nil {
		t.Fatal(err)
	}
	for _, sent := range frames[2:7] {
		got, err := readWorkerFrame(&output)
		if err != nil || got.kind != sent.kind || got.id != 4-sent.id || !bytes.Equal(got.body, sent.body) {
			t.Fatalf("frame %+v did not cross to the other attachment: %+v, %v", sent, got, err)
		}
	}
	if output.Len() != 0 {
		t.Fatal("unexpected additional worker output")
	}
}
