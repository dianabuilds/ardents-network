//go:build linux

package node

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestClosedIssuerHandlerReportsCurrentFactsFailure(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	events := make(chan Event, 1)
	config := runtimeConfig{Config: Config{
		Current: func() (DutyView, error) { return nil, errors.New("private current facts failure") },
		Emit:    func(_ context.Context, event Event) error { events <- event; return nil },
	}, now: time.Now}
	closedIssuerNodeHandler(config, tls.Certificate{}, nil, nil, nil)(t.Context(), route.ClosedSharedCarrier{Connection: server}, nil)
	select {
	case event := <-events:
		if event.Kind != "route-diagnostic" || event.Reason != "issuer-outer-facts-other" {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("issuer handler did not report its current-facts rejection")
	}
}

func TestAcceptClosedIssuerTLSReportsFixedEOF(t *testing.T) {
	certificate, _ := nodeCertificate(t, 243, "closed-issuer-rejection")
	report := make(chan string, 1)
	go func() {
		_, err := acceptClosedInnerTLS(t.Context(), closedIssuerEOFConn{}, certificate, time.Now().Add(time.Second))
		report <- closedIssuerInnerTLSFailureReason(err)
	}()
	select {
	case reason := <-report:
		if reason != "issuer-inner-tls-eof" {
			t.Fatalf("reason = %q", reason)
		}
	case <-time.After(time.Second):
		t.Fatal("issuer TLS acceptance did not terminate after peer close")
	}
}

type closedIssuerEOFConn struct{}

func (closedIssuerEOFConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (closedIssuerEOFConn) Write(value []byte) (int, error)  { return len(value), nil }
func (closedIssuerEOFConn) Close() error                     { return nil }
func (closedIssuerEOFConn) LocalAddr() net.Addr              { return closedIssuerEOFAddr{} }
func (closedIssuerEOFConn) RemoteAddr() net.Addr             { return closedIssuerEOFAddr{} }
func (closedIssuerEOFConn) SetDeadline(time.Time) error      { return nil }
func (closedIssuerEOFConn) SetReadDeadline(time.Time) error  { return nil }
func (closedIssuerEOFConn) SetWriteDeadline(time.Time) error { return nil }

type closedIssuerEOFAddr struct{}

func (closedIssuerEOFAddr) Network() string { return "test" }
func (closedIssuerEOFAddr) String() string  { return "issuer-eof" }
