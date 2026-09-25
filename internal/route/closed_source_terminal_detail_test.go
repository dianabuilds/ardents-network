//go:build linux

package route

import (
	"io"
	"testing"
	"time"
)

func TestClosedSourceTerminalDetailRetainsFirstCause(t *testing.T) {
	channels := newClosedSourceChannelOwner(nil, time.Now().Add(time.Minute), func() error { return nil })
	prefix := &ClosedSourcePrefix{channels: channels}
	channels.failAt("read", io.EOF)
	channels.stop()
	if got := prefix.TerminalDetail(); got != "read-eof" {
		t.Fatalf("terminal detail = %q, want first read EOF", got)
	}
}
