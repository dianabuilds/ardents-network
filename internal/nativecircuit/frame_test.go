package nativecircuit

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestFrameRoundTripAndBounds(t *testing.T) {
	t.Parallel()
	var stream bytes.Buffer
	want := frame{Type: frameIntroductionDeliver, Payload: []byte("opaque invitation")}
	if err := writeFrame(&stream, want); err != nil {
		t.Fatal(err)
	}
	got, err := readFrame(&stream)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != want.Type || !bytes.Equal(got.Payload, want.Payload) {
		t.Fatalf("frame differs: got %#v want %#v", got, want)
	}

	oversized := make([]byte, 4)
	binary.BigEndian.PutUint32(oversized, maximumFrameLength+1)
	if _, err := readFrame(bytes.NewReader(oversized)); err == nil {
		t.Fatal("oversized frame was accepted")
	}
	unknown := []byte{0, 0, 0, 1, 0xff}
	if _, err := readFrame(bytes.NewReader(unknown)); err == nil {
		t.Fatal("unknown frame type was accepted")
	}
}

func FuzzReadFrameFailsClosed(f *testing.F) {
	f.Add([]byte{0, 0, 0, 1, byte(frameClose)})
	f.Add([]byte{0, 1, 0, 1, byte(frameProtectedData)})
	f.Fuzz(func(t *testing.T, data []byte) {
		value, err := readFrame(bytes.NewReader(data))
		if err == nil {
			if !validFrameType(value.Type) || len(value.Payload)+1 > maximumFrameLength {
				t.Fatalf("parser accepted an out-of-contract frame: %#v", value)
			}
			if value.Type == frameProtectedData && len(value.Payload) > maximumApplicationPayload {
				t.Fatal("parser accepted an oversized Application frame")
			}
		}
	})
}
