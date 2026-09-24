//go:build linux

package node

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestAcceptClosedIssuerTLSReportsFixedEOF(t *testing.T) {
	certificate, _ := nodeCertificate(t, 243, "closed-issuer-rejection")
	report := make(chan string, 1)
	go func() {
		_, reason := acceptClosedIssuerTLS(t.Context(), closedIssuerEOFConn{}, certificate, time.Now().Add(time.Second))
		report <- reason
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
