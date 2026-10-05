//go:build linux

package transport

import (
	"context"
	"io"
	"net"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// These are mechanical post-admission controls. Genuine Network, token spend
// and Hosting acceptance are exercised by the separate command integration.
func TestJoinPairWaitsForBothActualResultsBeforeReadingData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		queues := &queueBudget{maximum: 64 << 20}
		pairs := &joinPairs{}
		var peers [2]net.Conn
		var observed [2]*joinReadObservation
		var capacities [2]*joinCapacity
		results := make(chan error, 2)
		for i := range 2 {
			local, remote := net.Pipe()
			peers[i] = remote
			defer remote.Close()
			observed[i] = &joinReadObservation{Conn: local}
			var err error
			capacities[i], err = reserveJoinCapacity(queues)
			if err != nil {
				t.Fatal(err)
			}
			h := ardp.Hello{ProfileDigest: [32]byte{3}, Purpose: ardp.PurposeDataJoin, Deadline: time.Now().Add(time.Minute).UTC().Truncate(time.Second)}
			r := ardp.JoinRequest{Nonce: [32]byte{byte(i + 1)}, Secret: [32]byte{4}, Context: [32]byte{5}, Side: uint8(i + 1), Deadline: time.Now().Add(10 * time.Second).UTC().Truncate(time.Second)}
			body, err := ardp.EncodeJoinRequest(r)
			if err != nil {
				t.Fatal(err)
			}
			go func() { results <- pairs.serve(ctx, observed[i], h, 32<<20, nil, capacities[i]) }()
			if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindOperation, Lane: 1, Body: body}); err != nil {
				t.Fatal(err)
			}
		}
		first, err := ardp.ReadFrame(peers[0])
		if err != nil {
			t.Fatal(err)
		}
		if first.Kind != ardp.KindResult || first.Lane != 1 {
			t.Fatal("missing local JOIN result")
		}
		if status, err := ardp.DecodeJoinResult(first.Body, [32]byte{1}); err != nil || status != 0 {
			t.Fatal("first local nonce or result differs", err)
		}
		synctest.Wait()
		before := observed[0].reads.Load()
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(peers[0], ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte("opaque")})
		}()
		synctest.Wait()
		if observed[0].reads.Load() != before {
			t.Fatal("first side read data before second actual RESULT write")
		}
		select {
		case err := <-written:
			t.Fatal("data entered before pair barrier", err)
		default:
		}
		second, err := ardp.ReadFrame(peers[1])
		if err != nil {
			t.Fatal(err)
		}
		if status, err := ardp.DecodeJoinResult(second.Body, [32]byte{2}); err != nil || status != 0 {
			t.Fatal("second local nonce or result differs", err)
		}
		data, err := ardp.ReadFrame(peers[1])
		if err != nil || data.Kind != ardp.KindBytes || data.Lane != 1 || string(data.Body) != "opaque" {
			t.Fatal("paired opaque data not forwarded", err)
		}
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		cancel()
		for range 2 {
			if err := <-results; err == nil || err == io.EOF {
				t.Fatal("cancellation became successful completion", err)
			}
		}
		for _, capacity := range capacities {
			capacity.release()
		}
		if queues.used != 0 || queues.children != 0 {
			t.Fatal("joined pair retained bounded memory or child capacity")
		}
	})
}
