package prefix

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/channel"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/route/transport/tls"
)

type recordPackingCounter struct {
	net.Conn
	frames atomic.Int32
}

func (c *recordPackingCounter) NetConn() net.Conn { return c.Conn }
func (c *recordPackingCounter) Write(raw []byte) (int, error) {
	if len(raw) >= ardp.HeaderSize && ardp.ValidHeader(raw[:ardp.HeaderSize]) && raw[6] == ardp.KindBytes {
		c.frames.Add(1)
	}
	return c.Conn.Write(raw)
}

// This isolates real TLS and framing mechanics. It supplies no Network duty,
// token or JOIN authority and is not a successful admitted Prefix scenario.
func TestNestedRoleRecordsDoNotMultiplyForwardingFrames(t *testing.T) {
	testNestedRecordPacking(t, false)
}

func TestLaneRecordsRespectEnclosingTLSCapacity(t *testing.T) {
	testNestedRecordPacking(t, true)
}

func testNestedRecordPacking(t *testing.T, laneWrite bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	left, right := net.Pipe()
	var sessions []*channel.Session
	t.Cleanup(func() {
		cancel()
		_ = left.Close()
		_ = right.Close()
		for _, session := range slices.Backward(sessions) {
			_ = session.Close()
		}
	})
	end := time.Now().Add(5 * time.Second)
	certificate, key := roleLaneCertificate(t)
	var counters []*recordPackingCounter
	client, server := left, right
	for layer := range 3 {
		if layer == 2 && laneWrite {
			// A relay writes opaque bytes directly to the Lane. There is no
			// local inner TLS writer to size them for the enclosing stack.
			break
		}
		type accepted struct {
			conn *tls.Conn
			err  error
		}
		ready := make(chan accepted, 1)
		go func(raw net.Conn) {
			secured, err := roletls.AcceptRole(ctx, raw, certificate, end)
			ready <- accepted{secured, err}
		}(server)
		secured, err := roletls.OpenRole(ctx, client, key, end)
		peer := <-ready
		if err != nil || peer.err != nil {
			t.Fatal("nested role handshake", layer, err, peer.err)
		}
		client, server = transport.Retain(secured), transport.Retain(peer.conn)
		_ = client.SetDeadline(end)
		_ = server.SetDeadline(end)
		if layer == 2 {
			break
		}
		counter := &recordPackingCounter{Conn: client}
		counters = append(counters, counter)
		a, outgoing, err := channel.PrepareJoined(ctx, counter, end, 1<<20, nil, channel.NewBudget(4<<20))
		if err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, a)
		b, incoming, err := channel.PrepareJoined(ctx, server, end, 1<<20, nil, channel.NewBudget(4<<20))
		if err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, b)
		a.Start()
		b.Start()
		client, server = outgoing, incoming
	}
	for _, counter := range counters {
		counter.frames.Store(0)
	}
	payload := bytes.Repeat([]byte{0x47}, ardp.HeaderSize+ardp.MaximumBodySize)
	received := make([]byte, len(payload))
	read := make(chan error, 1)
	go func() { _, err := io.ReadFull(server, received); read <- err }()
	n, writeErr := client.Write(payload)
	if readErr := <-read; writeErr != nil || readErr != nil || n != len(payload) || !bytes.Equal(received, payload) {
		t.Fatal("nested bytes changed", n, writeErr, readErr)
	}
	for layer, counter := range counters {
		if count := counter.frames.Load(); count != 2 {
			t.Errorf("forwarding layer %d sent %d BYTES frames, want two records without amplification", layer, count)
		}
	}
}
