package join

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// Post-admission physical relay only, with independent frame/byte oracles.
// No successful Network, Admission or Hosting authority is supplied here.
func TestJoinRelayDirectionalEOFBeforeTerminal(t *testing.T) {
	for _, data := range []bool{false, true} {
		name := "empty"
		if data {
			name = "after-duplex-and-returned-credit"
		}
		t.Run(name, func(t *testing.T) {
			f := newJoinBoundsFixture()
			defer f.close(t)
			peers := [2]*joinBoundsPeer{f.start(t, joinBoundsRequest(1, 1), nil), f.start(t, joinBoundsRequest(2, 2), nil)}
			for i, peer := range peers {
				if err := peer.conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
					t.Fatal(err)
				}
				readJoinBoundsResult(t, peer, uint8(i+1))
			}
			read := func(side int, kind uint8) ardp.Frame {
				t.Helper()
				frame, err := ardp.ReadFrame(peers[side].conn)
				if err != nil || frame.Kind != kind || frame.Lane != 1 {
					t.Fatal("original relay frame", side, kind, frame.Kind, err)
				}
				return frame
			}
			send := func(side int, frame ardp.Frame) <-chan error {
				done := make(chan error, 1)
				go func() { done <- ardp.WriteFrame(peers[side].conn, frame) }()
				return done
			}
			if data {
				for side := range 2 {
					payload := []byte("independent duplex payload")
					done := send(side, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: payload})
					frame := read(1-side, ardp.KindBytes)
					if !bytes.Equal(frame.Body, payload) {
						t.Fatal("payload changed")
					}
					if err := <-done; err != nil {
						t.Fatal(err)
					}
					credit := binary.BigEndian.AppendUint32(nil, uint32(len(payload)))
					done = send(1-side, ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: credit})
					frame = read(side, ardp.KindCredit)
					if !bytes.Equal(frame.Body, credit) {
						t.Fatal("credit changed")
					}
					if err := <-done; err != nil {
						t.Fatal(err)
					}
				}
			}
			first := send(0, ardp.Frame{Kind: ardp.KindEOF, Lane: 1})
			second := send(1, ardp.Frame{Kind: ardp.KindEOF, Lane: 1})
			for _, done := range []<-chan error{first, second} {
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			}
			read(0, ardp.KindEOF)
			read(1, ardp.KindEOF)
			done := send(0, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}})
			for side := range 2 {
				frame := read(side, ardp.KindClose)
				if !bytes.Equal(frame.Body, []byte{0}) {
					t.Fatal("terminal changed")
				}
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			for _, peer := range peers {
				if err := peer.wait(); err != nil {
					t.Fatal("original relay did not complete cleanly", err)
				}
			}
		})
	}
}
