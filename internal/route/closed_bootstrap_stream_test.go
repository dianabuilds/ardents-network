//go:build linux

package route

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func TestClosedBootstrapStreamConsumesParentRefillAccept(t *testing.T) {
	local, peer := net.Pipe()
	deadline := time.Now().Add(3 * time.Second)
	if err := local.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if err := peer.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	stream := newClosedRoleChildStream(local, deadline, local.Close, nil)
	body := make([]byte, 355)
	body[0] = 2
	refill := ardp.Frame{Kind: ardp.KindAdmit, Lane: 0, Body: body}
	served := make(chan error, 1)
	go func() {
		request, err := ardp.ReadFrame(peer)
		if err == nil && (request.Kind != ardp.KindAdmit || request.Lane != 0) {
			err = io.ErrUnexpectedEOF
		}
		if err == nil {
			var accepted ardp.Frame
			accepted, err = ardp.AcceptFrame(0, 64<<10)
			if err == nil {
				err = ardp.WriteFrame(peer, accepted)
			}
		}
		served <- err
	}()
	if err := stream.replenish(t.Context(), refill); err != nil {
		t.Fatal(err)
	}
	if err := <-served; err != nil {
		t.Fatal(err)
	}
	_ = peer.Close()
	_ = stream.Close()
}

func TestClosedBootstrapStreamEOFKeepsReverseCreditAlive(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	deadline := time.Now().Add(3 * time.Second)
	if err := local.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if err := peer.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	stream := newClosedRoleChildStream(local, deadline, local.Close, nil)
	defer stream.Close()
	allowCredit := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		for range 4 {
			if _, err := ardp.ReadFrame(peer); err != nil {
				finished <- err
				return
			}
		}
		if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindEOF, Lane: 1}); err != nil {
			finished <- err
			return
		}
		<-allowCredit
		if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: binary.BigEndian.AppendUint32(nil, 16)}); err != nil {
			finished <- err
			return
		}
		frame, err := ardp.ReadFrame(peer)
		if err == nil && (frame.Kind != ardp.KindBytes || len(frame.Body) != 16) {
			err = io.ErrUnexpectedEOF
		}
		finished <- err
	}()
	if n, err := stream.Write(make([]byte, 64<<10)); err != nil || n != 64<<10 {
		t.Fatalf("initial write: %d %v", n, err)
	}
	if _, err := stream.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("input EOF: %v", err)
	}
	close(allowCredit)
	if n, err := stream.Write(make([]byte, 16)); err != nil || n != 16 {
		t.Fatalf("write after input EOF: %d %v", n, err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}

func TestClosedBootstrapStreamDefersCreditUntilAuthenticatedHello(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	deadline := time.Now().Add(3 * time.Second)
	if err := local.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if err := peer.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	stream := newClosedRoleChildStream(local, deadline, local.Close, nil)
	defer stream.Close()
	delivered := make(chan error, 1)
	go func() {
		delivered <- ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{1, 2, 3}})
	}()
	if _, err := io.ReadFull(stream, make([]byte, 3)); err != nil {
		t.Fatal(err)
	}
	if err := <-delivered; err != nil {
		t.Fatal(err)
	}
	// Read above must finish without a peer reader accepting any pre-HELLO CREDIT.
	credit := make(chan ardp.Frame, 1)
	go func() { frame, _ := ardp.ReadFrame(peer); credit <- frame }()
	if err := stream.activate(); err != nil {
		t.Fatal(err)
	}
	frame := <-credit
	if frame.Kind != ardp.KindCredit || binary.BigEndian.Uint32(frame.Body) != 3 {
		t.Fatalf("deferred credit: %+v", frame)
	}
}
