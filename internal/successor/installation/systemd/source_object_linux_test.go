package systemd

import (
	"encoding/binary"
	"testing"
)

func sourceObjectReplyVector(owner, path string) []byte {
	var header []byte
	field := func(code byte, sig byte, body []byte) {
		for len(header)%8 != 0 {
			header = append(header, 0)
		}
		header = append(header, code, 1, sig, 0)
		header = append(header, body...)
	}
	field(5, 'u', binary.LittleEndian.AppendUint32(nil, 1))
	sender := binary.LittleEndian.AppendUint32(nil, uint32(len(owner)))
	sender = append(sender, owner...)
	sender = append(sender, 0)
	field(7, 's', sender)
	field(8, 'g', []byte{1, 'o', 0})
	body := binary.LittleEndian.AppendUint32(nil, uint32(len(path)))
	body = append(body, path...)
	body = append(body, 0)
	frame := []byte{'l', 2, 0, 1}
	frame = binary.LittleEndian.AppendUint32(frame, uint32(len(body)))
	frame = binary.LittleEndian.AppendUint32(frame, 1)
	frame = binary.LittleEndian.AppendUint32(frame, uint32(len(header)))
	frame = append(frame, header...)
	for len(frame)%8 != 0 {
		frame = append(frame, 0)
	}
	return append(frame, body...)
}

func TestSourceObjectReplyRequiresExactOriginalTypedPath(t *testing.T) {
	const path = "/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice"
	frame := sourceObjectReplyVector(":1.4", path)
	if e := verifyManagerFrame(frame, ":1.4", true, false); e != nil {
		t.Fatal("exact Source LoadUnit native reply", e)
	}
	if verifyManagerFrame(frame, ":1.4", false, false) == nil {
		t.Fatal("closed scalar-only reference generalized")
	}
	for _, bad := range [][]byte{sourceObjectReplyVector(":1.5", path), sourceObjectReplyVector(":1.4", "/org/freedesktop/systemd1/unit/ardents_2dendpoint_2eservice"), sourceObjectReplyVector(":1.4", path+"x"), sourceObjectReplyVector("org.freedesktop.DBus", path)} {
		if verifyManagerFrame(bad, ":1.4", true, false) == nil {
			t.Fatal("foreign/native object reply accepted")
		}
	}
	for length := 0; length < len(frame); length++ {
		if verifyManagerFrame(frame[:length], ":1.4", true, false) == nil {
			t.Fatal("truncated object reply accepted", length)
		}
	}
}
