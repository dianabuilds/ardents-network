package channel

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/route/ardp"
)

func TestIndependentLaneManyCreditWaitsPreserveBytes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local, peer := net.Pipe()
		s := New(t.Context(), local, time.Now().Add(time.Minute), 1<<20, nil, false, NewBudget(4<<20), nil)
		defer func() { _ = peer.Close(); _ = s.Close() }()
		lane := lifecycleLane(t, s, 1)
		s.mu.Lock()
		lane.credit = 0
		s.mu.Unlock()
		const total = 64
		body := bytes.Repeat([]byte{7}, total)
		written := make(chan error, 1)
		go func() { _, err := lane.Write(body); written <- err }()
		peerDone := make(chan struct{})
		go func() {
			defer close(peerDone)
			grant := func() bool {
				return ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: binary.BigEndian.AppendUint32(nil, 1)}) == nil
			}
			if !grant() {
				return
			}
			for received := 0; received < total; {
				f, err := ardp.ReadFrame(peer)
				if err != nil {
					return
				}
				if f.Kind == ardp.KindBytes {
					if !bytes.Equal(f.Body, bytes.Repeat([]byte{7}, len(f.Body))) {
						t.Error("peer received corrupted bytes")
					}
					received += len(f.Body)
				}
				if received < total && !grant() {
					return
				}
			}
		}()
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		<-peerDone
	})
}
