package prefix

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls"
)

// These are negative controls of the shared inner role TLS/result boundary.
// No State, Grant, Store ACK, successful publication or Carrier qualification
// is supplied by these fixtures; both-Carrier integrated success is separate.
func TestDescriptorActualTLSAndTruncatedResultRefuse(t *testing.T) {
	for _, fault := range []string{"tls-record", "truncated-result", "half-close"} {
		t.Run(fault, func(t *testing.T) {
			client, server, raw := descriptorResponseTLS(t)
			if fault == "half-close" {
				if err := client.NetConn().Close(); err != nil {
					t.Fatal(err)
				}
				if payload, err := finishDescriptorResponse(transport.Retain(client), bytes.NewReader(nil), nil, 0, func() error { return nil }); err == nil || len(payload) != 0 {
					t.Fatal("actual failed TLS half-close accepted", err)
				}
				return
			}
			completed := make(chan error, 1)
			go func() {
				if fault == "tls-record" {
					// A real invalid encrypted record on the actual authenticated
					// socket must fail TLS; no fake Read/Write result is injected.
					_, err := raw.Write([]byte{23, 3, 3, 0, 1, 0})
					completed <- err
					return
				}
				header := make([]byte, 16)
				copy(header, "ARDP")
				binary.BigEndian.PutUint16(header[4:6], 3)
				header[6] = 11
				binary.BigEndian.PutUint32(header[12:16], 16384)
				_, err := server.Write(append(header, 0))
				completed <- errors.Join(err, server.CloseWrite())
			}()
			if frame, err := ardp.ReadFrame(client); err == nil || len(frame.Body) != 0 {
				t.Fatal("actual TLS/truncated RESULT accepted", err)
			}
			if err := <-completed; err != nil {
				t.Fatal("fault sender did not physically finish", err)
			}
		})
	}
}

func descriptorResponseTLS(t *testing.T) (*tls.Conn, *tls.Conn, net.Conn) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(private) })
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	certificate, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	type acceptedRole struct {
		secured *tls.Conn
		raw     net.Conn
		err     error
	}
	accepted := make(chan acceptedRole, 1)
	end := now.Add(5 * time.Second)
	go func() {
		raw, err := listener.Accept()
		if err != nil {
			accepted <- acceptedRole{err: err}
			return
		}
		secured := tls.Server(raw, transport.RoleServerTLS(tls.Certificate{Certificate: [][]byte{certificate}, PrivateKey: private}))
		_ = secured.SetDeadline(end)
		accepted <- acceptedRole{secured: secured, raw: raw, err: secured.HandshakeContext(t.Context())}
	}()
	raw, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	var key [32]byte
	copy(key[:], public)
	client, err := roletls.OpenRole(t.Context(), raw, key, end)
	if err != nil {
		t.Fatal(err)
	}
	server := <-accepted
	if server.err != nil {
		t.Fatal(server.err)
	}
	t.Cleanup(func() { _ = server.raw.Close() })
	if err := client.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	return client, server.secured, server.raw
}

func TestDescriptorResponseRequiresOriginalTermination(t *testing.T) {
	client, server, _ := descriptorResponseTLS(t)
	finished := make(chan error, 1)
	go func() {
		var extra [1]byte
		n, err := server.Read(extra[:])
		if n != 0 || !errors.Is(err, io.EOF) {
			finished <- errors.New("actual client TLS half-close unavailable")
			return
		}
		finished <- server.CloseWrite()
	}()
	// Real peer TLS termination is still insufficient when the original lower
	// lane has trailing bytes. This unit supplies no successful Store ACK.
	if _, err := finishDescriptorResponse(transport.Retain(client), bytes.NewReader([]byte{1}), nil, 0, func() error { return nil }); err == nil {
		t.Fatal("trailing lower byte accepted")
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}
