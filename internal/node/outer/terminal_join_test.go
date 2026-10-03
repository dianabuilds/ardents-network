package outer

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// The transport fails an actual frame write, then holds physical retirement.
// A caller must not treat a returned write as joined while Close is still live.
type terminalCloseGate struct {
	net.Conn
	entered            chan struct{}
	resume             chan struct{}
	writeErr, closeErr error
}

func (connection *terminalCloseGate) Write([]byte) (int, error) {
	return 0, connection.writeErr
}

func (connection *terminalCloseGate) Close() error {
	close(connection.entered)
	<-connection.resume
	return connection.closeErr
}

func TestTerminalWriteJoinsFailedPhysicalRetirement(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	writeErr, closeErr := errors.New("terminal frame failure"), errors.New("physical retirement failure")
	connection := &terminalCloseGate{Conn: local, entered: make(chan struct{}), resume: make(chan struct{}), writeErr: writeErr, closeErr: closeErr}
	owner := &writer{connection: connection}
	completed := make(chan error, 1)
	go func() {
		completed <- owner.write(ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{1}},
			func() time.Time { return time.Now().Add(time.Second) }, true, true)
	}()
	select {
	case <-connection.entered:
	case <-time.After(time.Second):
		close(connection.resume)
		t.Fatal("terminal failure did not attempt physical retirement")
	}
	var result error
	early := false
	select {
	case result = <-completed:
		early = true
	case <-time.After(50 * time.Millisecond):
	}
	close(connection.resume)
	if !early {
		select {
		case result = <-completed:
		case <-time.After(time.Second):
			t.Fatal("joined physical retirement did not finish writer")
		}
	}
	if early {
		t.Error("terminal writer returned before physical retirement joined")
	}
	if !errors.Is(result, writeErr) || !errors.Is(result, closeErr) {
		t.Errorf("terminal result lost write or physical-close cause: %v", result)
	}
	if retained := owner.result(); !errors.Is(retained, writeErr) || !errors.Is(retained, closeErr) {
		t.Errorf("Carrier cleanup lost physical writer failure: %v", retained)
	}
}
