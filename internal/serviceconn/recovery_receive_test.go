package serviceconn

import (
	"bytes"
	"errors"
	"testing"
)

func TestReceiveRangesDeliverOnceInOrderAndRejectConflicts(t *testing.T) {
	application := &bufferApplication{}
	stream := &recoveryStream{application: application}
	if err := stream.acceptData(nil, connectionFrame{offset: 3, data: []byte("def")}, 6); err != nil {
		t.Fatal(err)
	}
	if application.Len() != 0 {
		t.Fatal("out-of-order bytes reached the Application")
	}
	if err := stream.acceptData(nil, connectionFrame{offset: 0, data: []byte("abc")}, 6); err != nil {
		t.Fatal(err)
	}
	if application.String() != "abcdef" || stream.recvNext != 6 {
		t.Fatalf("ranges were not delivered once in order: %q offset=%d", application.String(), stream.recvNext)
	}
	if err := stream.acceptData(nil, connectionFrame{offset: 2, data: []byte("cde")}, 6); err != nil {
		t.Fatalf("matching delayed overlap rejected: %v", err)
	}
	if application.String() != "abcdef" {
		t.Fatal("matching overlap was presented twice")
	}
	if err := stream.acceptData(nil, connectionFrame{offset: 2, data: []byte("Xde")}, 6); !errors.Is(err, errActiveViolation) {
		t.Fatalf("conflicting authenticated overlap accepted: %v", err)
	}
}

func TestReceiveRangeMetadataStopsAtEightDisjointRanges(t *testing.T) {
	stream := &recoveryStream{application: &bufferApplication{}}
	for index := range 8 {
		offset := uint64(index*2 + 1)
		if err := stream.acceptData(nil, connectionFrame{offset: offset, data: []byte{byte(index)}}, 32); err != nil {
			t.Fatalf("range %d rejected: %v", index+1, err)
		}
	}
	if len(stream.pending) != 8 {
		t.Fatalf("wrong disjoint range count: %d", len(stream.pending))
	}
	if err := stream.acceptData(nil, connectionFrame{offset: 17, data: []byte{9}}, 32); !errors.Is(err, errActiveViolation) {
		t.Fatalf("ninth disjoint range did not terminate safely: %v", err)
	}
}

type bufferApplication struct{ bytes.Buffer }

func (*bufferApplication) Close() error { return nil }
