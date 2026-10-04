//go:build linux

package transport

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/carrier"
)

// This is actual nested TLS over ARDP lanes, solely a framing oracle. No
// HELLO/token/Grant or successful domain authority is supplied by the fixture.
func nestedRetirementTLS(t *testing.T) (*lane, *lane, *tls.Conn) {
	t.Helper()
	local, peer := net.Pipe()
	end := time.Now().Add(10 * time.Second)
	a := newSession(t.Context(), local, end, 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	b := newSession(t.Context(), peer, end, 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
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
		_, err := carrier.AcceptClosedRoleTLS(t.Context(), bl, tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, end)
		accepted <- err
	}()
	var pinned [32]byte
	copy(pinned[:], public)
	secured, err := carrier.OpenClosedRoleTLS(t.Context(), al, pinned, end)
	if err != nil {
		t.Fatal(err)
	}
	if err := lifecycleResult(t, accepted); err != nil {
		t.Fatal(err)
	}
	return al, bl, secured
}

func waitFramingState(t *testing.T, l *lane, want func() bool) {
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

func TestNestedTLSUnemittedControlRequiresActualPeerCLOSE(t *testing.T) {
	for _, kind := range []uint8{ardp.KindCredit, ardp.KindClose} {
		for _, mode := range []string{"clean", "refused", "raw-eof", "local-close"} {
			t.Run(string(rune('0'+kind))+"/"+mode, func(t *testing.T) {
				lower, peer, secured := nestedRetirementTLS(t)
				ctx, cancel := context.WithCancel(t.Context())
				// Write-only upper owner: there is no fabricated successful reader.
				upper := &session{conn: &retiredConn{Conn: secured}, ctx: ctx, cancel: cancel, end: time.Now().Add(5 * time.Second), lanes: make(map[uint32]*lane), next: 1, limit: 32 << 20, writer: make(chan struct{}, 1), opening: make(chan struct{}, 1), queues: &queueBudget{maximum: 64 << 20}, readerDone: make(chan struct{})}
				close(upper.readerDone)
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
				if mode == "clean" && upper.joinedPhysicalFailure() != nil {
					t.Fatal("wholly-unemitted control marked physical", upper.joinedPhysicalFailure())
				}
				if mode != "clean" {
					if upper.Close() == nil {
						t.Fatal("unemitted refused/local/raw closure became channel success")
					}
					if upper.joinedPhysicalFailure() != nil {
						t.Fatal("unemitted terminal refusal became physical output failure", upper.joinedPhysicalFailure())
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
