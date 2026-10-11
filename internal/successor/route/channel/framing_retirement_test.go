package channel

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"math/big"
	"net"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// This is actual nested TLS over ARDP lanes, solely a framing oracle. No
// HELLO/token/Grant or successful domain authority is supplied by the fixture.
func nestedRetirementTLS(t *testing.T) (*Lane, *Lane, *tls.Conn) {
	t.Helper()
	local, peer := net.Pipe()
	end := time.Now().Add(10 * time.Second)
	a := New(t.Context(), local, end, 32<<20, nil, false, &Budget{maximum: 64 << 20}, nil)
	b := New(t.Context(), peer, end, 32<<20, nil, false, &Budget{maximum: 64 << 20}, nil)
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
	al, bl := lifecycleLane(t, a, 1), lifecycleLane(t, b, 1)
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Minute), NotAfter: end, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, public, key)
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan error, 1)
	go func() {
		server := tls.Server(bl, transport.RoleServerTLS(tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}))
		err := server.SetDeadline(end)
		if err == nil {
			err = server.HandshakeContext(t.Context())
		}
		if err == nil {
			err = server.SetDeadline(time.Time{})
		}
		accepted <- err
	}()
	var pinned [32]byte
	copy(pinned[:], public)
	secured := tls.Client(al, transport.RoleClientTLS(pinned))
	err = secured.SetDeadline(end)
	if err == nil {
		err = secured.HandshakeContext(t.Context())
	}
	if err == nil {
		err = secured.SetDeadline(time.Time{})
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := lifecycleResult(t, accepted); err != nil {
		t.Fatal(err)
	}
	return al, bl, secured
}

func waitFramingState(t *testing.T, l *Lane, want func() bool) {
	t.Helper()
	until := time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		l.s.mu.Lock()
		ready := want()
		l.s.mu.Unlock()
		if ready {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("framing state did not reach its causal boundary")
}

func TestNestedTLSDeadlineRefusalRetainsTLSFailureAndHealthyAncestor(t *testing.T) {
	lower, peer, secured := nestedRetirementTLS(t)
	before := lower.retirementWitness()
	if err := lower.SetWriteDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	n, first := secured.Write([]byte("refused TLS record"))
	if n != 0 || !errors.Is(first, os.ErrDeadlineExceeded) {
		t.Fatal("TLS write did not encounter the local deadline", n, first)
	}
	if err := lower.SetWriteDeadline(lower.end); err != nil {
		t.Fatal(err)
	}
	n, second := secured.Write([]byte("same TLS generation"))
	if n != 0 || second != first {
		t.Fatal("damaged TLS writer lost its retained failure", n, first, second)
	}
	after := lower.retirementWitness()
	if after.payload != before.payload || after.attempts != before.attempts {
		t.Fatal("local TLS refusal produced lower physical output")
	}
	// Another lane on the actual unaffected framing ancestor remains usable.
	sibling := lifecycleLane(t, lower.s, 3)
	peerSibling := lifecycleLane(t, peer.s, 3)
	read := make(chan error, 1)
	go func() {
		buffer := make([]byte, len("healthy sibling"))
		n, err := peerSibling.Read(buffer)
		if err == nil && (n != len(buffer) || string(buffer) != "healthy sibling") {
			err = errors.New("sibling bytes differ")
		}
		read <- err
	}()
	if _, err := sibling.Write([]byte("healthy sibling")); err != nil {
		t.Fatal("local TLS failure retired healthy ancestor", err)
	}
	if err := lifecycleResult(t, read); err != nil {
		t.Fatal(err)
	}
}

func TestNestedTLSUnemittedControlRequiresActualPeerCLOSE(t *testing.T) {
	for _, kind := range []uint8{ardp.KindCredit, ardp.KindClose} {
		for _, mode := range []string{"clean", "refused", "raw-eof", "local-close"} {
			t.Run(string(rune('0'+kind))+"/"+mode, func(t *testing.T) {
				lower, peer, secured := nestedRetirementTLS(t)
				ctx, cancel := context.WithCancel(t.Context())
				// Write-only upper owner: there is no fabricated successful reader.
				upper := &Session{conn: transport.Retain(secured), ctx: ctx, cancel: cancel, end: time.Now().Add(5 * time.Second), lanes: make(map[uint32]*Lane), next: 1, limit: 32 << 20, writer: make(chan struct{}, 1), opening: make(chan struct{}, 1), queues: &Budget{maximum: 64 << 20}, readerDone: make(chan struct{})}
				upper.readerOnce.Do(func() { close(upper.readerDone) })
				defer upper.Close()
				child := lifecycleLane(t, upper, 1)
				child.cleanupEnd = time.Now().Add(time.Second)
				body := []byte{0, 0, 0, 1}
				if kind == ardp.KindClose {
					body = []byte{0}
				}
				lower.writeMu.Lock()
				gateHeld := true
				defer func() {
					if gateHeld {
						lower.writeMu.Unlock()
					}
				}()
				before := lower.retirementWitness()
				written := make(chan error, 1)
				go func() {
					written <- upper.write(child, ardp.Frame{Kind: kind, Lane: child.id, Body: body}, kind == ardp.KindClose)
				}()
				waitFramingState(t, child, func() bool { return upper.active == child })
				if mode == "raw-eof" {
					_ = peer.s.conn.Close()
					waitFramingState(t, lower, func() bool { return lower.s.stopped })
				} else {
					status := byte(0)
					if mode == "refused" {
						status = 1
					}
					peer.cleanupEnd = time.Now().Add(time.Second)
					if err := peer.s.write(peer, ardp.Frame{Kind: ardp.KindClose, Lane: peer.id, Body: []byte{status}}, true); err != nil {
						t.Fatal(err)
					}
					waitFramingState(t, lower, func() bool { return lower.peerClosed })
				}
				var localDone chan error
				if mode == "local-close" {
					localDone = make(chan error, 1)
					go func() { localDone <- lower.Close() }()
					waitFramingState(t, lower, func() bool { return lower.localClosed })
				}
				lower.writeMu.Unlock()
				gateHeld = false
				err := lifecycleResult(t, written)
				if (mode == "clean") != (err == nil) {
					t.Fatal("wrong exact nested-control result", mode, err)
				}
				after := lower.retirementWitness()
				if before.payload != after.payload {
					t.Fatal("supposed wholly-unemitted TLS control produced lower payload")
				}
				if mode == "clean" && upper.PhysicalFailure() != nil {
					t.Fatal("wholly-unemitted control marked physical", upper.PhysicalFailure())
				}
				if mode != "clean" {
					if upper.Close() == nil {
						t.Fatal("unemitted refused/local/raw closure became channel success")
					}
					if upper.PhysicalFailure() != nil {
						t.Fatal("unemitted terminal refusal became physical output failure", upper.PhysicalFailure())
					}
				}
				if localDone != nil {
					if err := lifecycleResult(t, localDone); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

type physicalDeadlineFailureConn struct {
	*lifecycleConn
	failure error
}

func (c *physicalDeadlineFailureConn) SetWriteDeadline(end time.Time) error {
	c.mu.Lock()
	err := c.failure
	c.mu.Unlock()
	if err != nil {
		return err
	}
	return c.lifecycleConn.SetWriteDeadline(end)
}

func TestLowerPhysicalDeadlineFailureRemainsWithItsOwner(t *testing.T) {
	for _, operation := range []string{"deadline", "interrupt"} {
		t.Run(operation, func(t *testing.T) {
			physical := &physicalDeadlineFailureConn{lifecycleConn: newLifecycleConn(true)}
			physical.writeIgnoresClose = true
			end := time.Now().Add(5 * time.Second)
			s := New(t.Context(), physical, end, 32<<20, nil, false, &Budget{maximum: 64 << 20}, nil)
			defer s.Close()
			l := lifecycleLane(t, s, 1)
			written := make(chan error, 1)
			go func() { _, err := l.Write([]byte("original lower output")); written <- err }()
			select {
			case <-physical.writes:
			case <-time.After(time.Second):
				t.Fatal("lower physical writer did not start")
			}
			failure := errors.New("actual lower deadline operation failed")
			physical.mu.Lock()
			physical.failure = failure
			physical.mu.Unlock()
			interrupt := func() error { return l.SetWriteDeadline(end) }
			if operation == "interrupt" {
				interrupt = l.InterruptIO
			}
			if err := interrupt(); err != failure {
				t.Fatal("physical deadline refusal changed", err)
			}
			s.Retire(failure)
			close(physical.writeGate)
			_ = lifecycleResult(t, written)
			if err := s.Close(); !errors.Is(err, failure) {
				t.Fatal("joined lower failure lost", err)
			}
			if err := s.PhysicalFailure(); !errors.Is(err, failure) {
				t.Fatal("actual lower deadline failure was treated as an unemitted upper refusal", err)
			}
		})
	}
}

func TestNestedDeadlineRefusalAfterLowerRetirementIsNotUpperPhysicalFailure(t *testing.T) {
	physical := &retiredDeadlineConn{newLifecycleConn(true)}
	physical.writeIgnoresClose = true
	late := errors.New("lower original writer failed late")
	physical.partial = late
	end := time.Now().Add(5 * time.Second)
	lowerOwner := New(t.Context(), physical, end, 32<<20, nil, false, &Budget{maximum: 64 << 20}, nil)
	defer lowerOwner.Close()
	lower := lifecycleLane(t, lowerOwner, 1)
	lowerWritten := make(chan error, 1)
	go func() { _, err := lower.Write([]byte("original lower frame")); lowerWritten <- err }()
	select {
	case <-physical.writes:
	case <-time.After(time.Second):
		t.Fatal("original lower writer did not start")
	}
	authority := errors.New("original lower owner retired")
	lowerOwner.Retire(authority)
	ctx, cancel := context.WithCancel(t.Context())
	upper := &Session{conn: transport.Retain(lower), ctx: ctx, cancel: cancel, end: end, lanes: make(map[uint32]*Lane), next: 1, limit: 32 << 20, writer: make(chan struct{}, 1), opening: make(chan struct{}, 1), queues: &Budget{maximum: 64 << 20}, readerDone: make(chan struct{})}
	upper.readerOnce.Do(func() { close(upper.readerDone) })
	defer upper.Close()
	child := lifecycleLane(t, upper, 1)
	upperWritten := make(chan error, 1)
	go func() {
		upperWritten <- upper.write(child, ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: []byte{0, 0, 0, 1}}, false)
	}()
	waitFramingState(t, child, func() bool { return upper.stopped })
	close(physical.writeGate)
	if err := lifecycleResult(t, lowerWritten); !errors.Is(err, late) {
		t.Fatal("lower physical failure lost", err)
	}
	if err := lifecycleResult(t, upperWritten); !errors.Is(err, net.ErrClosed) {
		t.Fatal("upper control did not retain the lower refusal", err)
	}
	if err := upper.Close(); err == nil {
		t.Fatal("unemitted upper refusal became success")
	}
	if err := upper.PhysicalFailure(); err != nil {
		t.Fatal("lower deadline refusal fabricated upper physical output failure", err)
	}
	if err := lowerOwner.Close(); !errors.Is(err, authority) || !errors.Is(err, late) || errors.Is(err, net.ErrClosed) {
		t.Fatal("lower original joined provenance changed", err)
	}
}

func TestFramingRetirementWitnessVetoesActualProgressAndFailures(t *testing.T) {
	before := framingWitness{attempts: 4, payload: 3}
	clean := framingWitness{attempts: 4, payload: 3, clean: true}
	if !cleanUnemittedRetirement(ardp.KindCredit, before, clean) || !cleanUnemittedRetirement(ardp.KindClose, before, clean) {
		t.Fatal("exact unchanged live CLOSE witness refused")
	}
	for _, after := range []framingWitness{{attempts: 5, payload: 4, clean: true}, {attempts: 4, payload: 3, busy: true, clean: true}, {attempts: 4, payload: 3, clean: false}} {
		if cleanUnemittedRetirement(ardp.KindCredit, before, after) || cleanUnemittedRetirement(ardp.KindClose, before, after) {
			t.Fatal("changed/active/failed lower writer acquired clean witness", after)
		}
	}
	creditOnly := framingWitness{attempts: 5, payload: 3, clean: true}
	if cleanUnemittedRetirement(ardp.KindCredit, before, creditOnly) || !cleanUnemittedRetirement(ardp.KindClose, before, creditOnly) {
		t.Fatal("CREDIT and non-CREDIT attempt namespaces collapsed")
	}
	before.local = true
	if cleanUnemittedRetirement(ardp.KindCredit, before, clean) || cleanUnemittedRetirement(ardp.KindClose, before, clean) {
		t.Fatal("preexisting local close manufactured peer retirement")
	}
}
