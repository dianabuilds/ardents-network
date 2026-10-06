package transport

import (
	"errors"
	"net"
	"testing"
)

type halfCloseProbe struct {
	net.Conn
	halfCloses, closes int
	halfErr, closeErr  error
}

func (c *halfCloseProbe) CloseWrite() error { c.halfCloses++; return c.halfErr }
func (c *halfCloseProbe) Close() error      { c.closes++; return c.closeErr }

func TestRetainPreservesHalfCloseAndJoinedCloseFailure(t *testing.T) {
	first, second := net.Pipe()
	defer first.Close()
	defer second.Close()
	halfErr, closeErr := errors.New("half-close failure"), errors.New("physical close failure")
	probe := &halfCloseProbe{Conn: first, halfErr: halfErr, closeErr: closeErr}
	retained := Retain(probe)
	writer, ok := retained.(interface{ CloseWrite() error })
	if !ok {
		t.Fatal("retained connection lost half-close capability")
	}
	if err := writer.CloseWrite(); !errors.Is(err, halfErr) || probe.halfCloses != 1 || probe.closes != 0 {
		t.Fatal("half-close changed physical ownership", err, probe.halfCloses, probe.closes)
	}
	if Retain(retained) != retained {
		t.Fatal("retention acquired a second owner")
	}
	for range 2 {
		if err := retained.Close(); !errors.Is(err, closeErr) {
			t.Fatal("lost original physical failure", err)
		}
	}
	if probe.closes != 1 {
		t.Fatal("physical close repeated", probe.closes)
	}
	if _, ok := Retain(second).(interface{ CloseWrite() error }); ok {
		t.Fatal("retention fabricated unsupported half-close")
	}
}
