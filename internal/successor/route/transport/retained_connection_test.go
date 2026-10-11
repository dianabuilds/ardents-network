package transport

import (
	"errors"
	"io"
	"net"
	"testing"
)

type halfCloseProbe struct {
	net.Conn
	halfCloses, closes int
	halfErr, closeErr  error
}

func TestRetainedPlainReadCannotAttestPeerTLSEOF(t *testing.T) {
	local, remote := net.Pipe()
	retained := Retain(local)
	defer retained.Close()
	if err := remote.Close(); err != nil {
		t.Fatal(err)
	}
	var extra [1]byte
	if n, err := retained.Read(extra[:]); n != 0 || err != io.EOF {
		t.Fatal("original pipe EOF changed", n, err)
	}
	if got := RetainedPeerReadCause(retained, io.EOF); got != io.EOF || IsPeerRetirementCause(got) {
		t.Fatal("plain stream manufactured authenticated TLS read provenance", got)
	}
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
