package systemd

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// Reference-frame vectors use the production decoder's scalar-only vocabulary.
func verifyReferenceFrame(frame []byte, owner string) error {
	return verifyManagerFrame(frame, owner, false, false, sourceUnitReply)
}

func TestReadOnlyManagerReplyCannotBroadenReferenceAdmission(t *testing.T) {
	body := managerDictionaryVector("Original")
	frame := make([]byte, 16)
	frame[0], frame[1], frame[3] = 'l', 2, 1
	frame = append(frame, 5, 1, 'u', 0, 1, 0, 0, 0)
	frame = append(frame, 7, 1, 's', 0, 4, 0, 0, 0, ':', '1', '.', '2', 0)
	for len(frame)%8 != 0 {
		frame = append(frame, 0)
	}
	frame = append(frame, 8, 1, 'g', 0, 5, 'a', '{', 's', 'v', '}', 0)
	binary.LittleEndian.PutUint32(frame[12:16], uint32(len(frame)-16))
	for len(frame)%8 != 0 {
		frame = append(frame, 0)
	}
	frame = append(frame, body...)
	binary.LittleEndian.PutUint32(frame[4:8], uint32(len(body)))
	if err := verifyManagerFrame(frame, ":1.2", true, false, sourceUnitReply); err != nil {
		t.Fatal("independent original property reply refused", err)
	}
	if err := verifyReferenceFrame(frame, ":1.2"); err == nil {
		t.Fatal("property vocabulary expanded the scalar reference client")
	}
	if err := verifyManagerFrame(frame, ":1.3", true, false, sourceUnitReply); err == nil {
		t.Fatal("foreign manager supplied original observations")
	}
	oversized := bytes.Clone(frame)
	binary.LittleEndian.PutUint32(oversized[len(frame)-len(body):], ^uint32(0))
	if err := verifyManagerFrame(oversized, ":1.2", true, false, sourceUnitReply); err == nil {
		t.Fatal("original property frame admitted unchecked nested allocation")
	}
}
