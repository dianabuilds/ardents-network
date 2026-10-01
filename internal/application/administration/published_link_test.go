package administration

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

type publishedLinkTestOwner struct {
	testInterface
	link    string
	failure error
	calls   atomic.Int32
}

func (owner *publishedLinkTestOwner) PublishedLink(context.Context) (string, error) {
	owner.calls.Add(1)
	return owner.link, owner.failure
}

func TestPublishedLinkWireUsesExactLengthAndDirectionalEOF(t *testing.T) {
	owner := &publishedLinkTestOwner{link: "ardents://bounded-public-destination"}
	path, server := snapshotTestServer(t, owner)
	defer func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	}()
	connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connection.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(connection, "link\n"); err != nil {
		t.Fatal(err)
	}
	if err := connection.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(connection)
	expected := append([]byte("link\n\x00\x24"), []byte("ardents://bounded-public-destination")...)
	if err != nil || !bytes.Equal(raw, expected) {
		t.Fatalf("published Link wire = %x, %v", raw, err)
	}
}
