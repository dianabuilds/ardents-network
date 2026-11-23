package document_test

import (
	"bytes"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/document"
)

func TestSnapshotServesOneImmutableDocument(t *testing.T) {
	original := []byte("hello\n")
	snapshot, err := document.NewSnapshot(original)
	if err != nil {
		t.Fatal(err)
	}
	original[0] = 'X'
	request := make([]byte, 512)
	copy(request, "ARDTXT01\x01")
	for range 2 {
		var response bytes.Buffer
		if err := snapshot.Respond(bytes.NewReader(request), &response); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(response.Bytes(), []byte("ARDTXT01\x00\x00\x00\x00\x06hello\n")) {
			t.Fatalf("response = %q", response.Bytes())
		}
	}
}

func TestSnapshotRejectsMalformedAndRepeatedRequestsBeforeResponse(t *testing.T) {
	snapshot, err := document.NewSnapshot([]byte("private document"))
	if err != nil {
		t.Fatal(err)
	}
	valid := make([]byte, 512)
	copy(valid, "ARDTXT01\x01")
	padding := bytes.Clone(valid)
	padding[511] = 1
	for _, request := range [][]byte{nil, valid[:511], append(bytes.Clone(valid), 0), append(bytes.Clone(valid), valid...), padding} {
		var response bytes.Buffer
		if err := snapshot.Respond(bytes.NewReader(request), &response); err == nil || response.Len() != 0 {
			t.Fatalf("invalid request emitted a response: bytes=%d error=%v", response.Len(), err)
		}
	}
	for _, body := range [][]byte{{0xff}, bytes.Repeat([]byte{'a'}, document.MaximumBytes+1)} {
		if _, err := document.NewSnapshot(body); err == nil {
			t.Fatal("invalid snapshot accepted")
		}
	}
}
