//go:build linux

package route

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

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
			if _, err := ReadClosedLaneFrame(peer); err != nil {
				finished <- err
				return
			}
		}
		if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1}); err != nil {
			finished <- err
			return
		}
		<-allowCredit
		if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameCredit, Lane: 1, Body: binary.BigEndian.AppendUint32(nil, 16)}); err != nil {
			finished <- err
			return
		}
		frame, err := ReadClosedLaneFrame(peer)
		if err == nil && (frame.Kind != closedFrameBytes || len(frame.Body) != 16) {
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
		delivered <- WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{1, 2, 3}})
	}()
	if _, err := io.ReadFull(stream, make([]byte, 3)); err != nil {
		t.Fatal(err)
	}
	if err := <-delivered; err != nil {
		t.Fatal(err)
	}
	// Read above must finish without a peer reader accepting any pre-HELLO CREDIT.
	credit := make(chan ClosedLaneFrame, 1)
	go func() { frame, _ := ReadClosedLaneFrame(peer); credit <- frame }()
	if err := stream.activate(); err != nil {
		t.Fatal(err)
	}
	frame := <-credit
	if frame.Kind != closedFrameCredit || binary.BigEndian.Uint32(frame.Body) != 3 {
		t.Fatalf("deferred credit: %+v", frame)
	}
}
