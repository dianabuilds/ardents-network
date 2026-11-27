package systemd

import (
	"encoding/binary"
	"testing"
)

func TestManagerReferenceAcceptsOnlyItsFixedUnitObject(t *testing.T) {
	const unit = "/org/freedesktop/systemd1/unit/ardents_2d"
	fixed := []struct {
		name   string
		reply  managerUnitReply
		object string
	}{
		{"Source", sourceUnitReply, unit + "source_2eservice"},
		{"Forwarding", forwardingUnitReply, unit + "forwarding_2eservice"},
		{"JOIN", joinUnitReply, unit + "join_2eservice"},
		{"Descriptor", descriptorUnitReply, unit + "descriptor_2eservice"},
		{"Introduction", introductionUnitReply, unit + "introduction_2eservice"},
		{"absence", nodeSlotsReply, ""},
	}
	foreign := []string{
		"", "/", unit + "issuer_2eservice", unit + "endpoint_2eservice",
		"/org/freedesktop/systemd1/job/1", "/org/freedesktop/systemd1/job/01",
		"/org/freedesktop/systemd1/job/4294967295", "/org/freedesktop/systemd1/job/4294967296",
	}
	for _, entry := range fixed {
		if entry.object != "" {
			foreign = append(foreign, entry.object)
		}
	}
	const manager = ":1.4"
	for _, entry := range fixed {
		t.Run(entry.name, func(t *testing.T) {
			for _, object := range foreign {
				frame := managerObjectReplyVector(manager, object)
				err := verifyManagerFrame(frame, manager, true, entry.reply)
				if (err == nil) != (object != "" && object == entry.object) {
					t.Errorf("fixed object %q: %v", object, err)
				}
				if verifyManagerFrame(frame, ":1.5", true, entry.reply) == nil {
					t.Errorf("foreign original manager supplied object %q", object)
				}
			}
		})
	}
}

func managerObjectReplyVector(owner, path string) []byte {
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
	frame := managerObjectReplyVector(":1.4", path)
	if e := verifyManagerFrame(frame, ":1.4", true, sourceUnitReply); e != nil {
		t.Fatal("exact Source LoadUnit native reply", e)
	}
	if verifyManagerFrame(frame, ":1.4", false, sourceUnitReply) == nil {
		t.Fatal("closed scalar-only reference generalized")
	}
	for _, bad := range [][]byte{managerObjectReplyVector(":1.5", path), managerObjectReplyVector(":1.4", "/org/freedesktop/systemd1/unit/ardents_2dendpoint_2eservice"), managerObjectReplyVector(":1.4", path+"x"), managerObjectReplyVector("org.freedesktop.DBus", path)} {
		if verifyManagerFrame(bad, ":1.4", true, sourceUnitReply) == nil {
			t.Fatal("foreign/native object reply accepted")
		}
	}
	for length := 0; length < len(frame); length++ {
		if verifyManagerFrame(frame[:length], ":1.4", true, sourceUnitReply) == nil {
			t.Fatal("truncated object reply accepted", length)
		}
	}
}
