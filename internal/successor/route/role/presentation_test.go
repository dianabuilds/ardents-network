package role

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// Actual TLS and supplied-fact observations isolate refusal before ADMIT.
// The placeholder bytes below never become an emitted token or accepted Grant;
// successful Stock/spend and both Carrier scenarios remain command tests.
func TestPresentationOriginalCallerCancellationPreventsAdmit(t *testing.T) {
	client, server, interrupt := presentationTLS(t)
	now := time.Now().UTC().Truncate(time.Second)
	view := authorityObservation(t, now, 1, [32]byte{104})
	duty, err := view.RetainDuty([32]byte{1}, now)
	if err != nil {
		t.Fatal(err)
	}
	a := Authority{Duty: duty, Profile: view.Profile().ProfileBinding, Current: func() (network.RuntimeView, error) { return view, nil }}
	hello, err := a.FreshHello(now.Add(time.Minute), ardp.PurposeForwarding, false)
	if err != nil {
		t.Fatal(err)
	}
	caller, cancel := context.WithCancel(t.Context())
	defer cancel()
	peer := make(chan error, 1)
	go func() {
		got, err := channel.ReadHello(server)
		if err != nil {
			peer <- err
			return
		}
		if got != hello {
			peer <- errors.New("HELLO changed")
			return
		}
		frame, err := ardp.ReadFrame(server)
		if err == nil || frame.Kind == ardp.KindAdmit {
			peer <- errors.New("ADMIT emitted after original caller cancellation")
			return
		}
		peer <- nil
	}()
	placeholder := bytes.Repeat([]byte{99}, 354)
	presentations := 0
	err = Present(t.Context(), caller, client, a, hello, func(context.Context, ardp.Hello) ([]byte, error) {
		presentations++
		cancel() // Derived physical context deliberately remains live.
		return placeholder, nil
	})
	if !errors.Is(err, context.Canceled) || presentations != 1 {
		t.Fatal("original caller loss not retained", err, presentations)
	}
	if !bytes.Equal(placeholder, make([]byte, len(placeholder))) {
		t.Fatal("refused presentation bytes retained")
	}
	interrupt()
	if err := <-peer; err != nil {
		t.Fatal(err)
	}
}

func TestBindingRequiresNegotiatedRoleExporter(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	view := authorityObservation(t, now, 1, [32]byte{104})
	duty, err := view.RetainDuty([32]byte{1}, now)
	if err != nil {
		t.Fatal(err)
	}
	a := Authority{Duty: duty, Profile: view.Profile().ProfileBinding, Current: func() (network.RuntimeView, error) { return view, nil }}
	h, err := a.FreshHello(now.Add(time.Minute), ardp.PurposeForwarding, false)
	if err != nil {
		t.Fatal(err)
	}
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()
	if binding, err := Binding(local, h); err == nil || binding != [32]byte{} {
		t.Fatal("plain stream supplied an accepting role binding", binding, err)
	}
	called := false
	if err := Present(t.Context(), t.Context(), local, a, h, func(context.Context, ardp.Hello) ([]byte, error) {
		called = true
		return nil, errors.New("unreachable presentation")
	}); err == nil || called {
		t.Fatal("missing exporter reached presentation", err, called)
	}
}

func TestPurposeAdmissionClassKeepsExistingBounds(t *testing.T) {
	for _, purpose := range []ardp.Purpose{ardp.PurposeForwarding, ardp.PurposeDataJoin, ardp.PurposeIntroduction, ardp.PurposeIssuer} {
		class, err := AdmissionClass(purpose)
		if err != nil {
			t.Fatal(err)
		}
		want, limit := admission.ForwardClass, uint64(33554432)
		if purpose == ardp.PurposeIntroduction {
			want, limit = admission.RegistrationClass, 1048576
		}
		if purpose == ardp.PurposeIssuer {
			want, limit = admission.ControlClass, 65536
			if class.Lifetime() != 30*time.Second {
				t.Fatal("issuer Control horizon changed", class.Lifetime())
			}
		}
		if class != want || class.ByteLimit() != limit {
			t.Fatal("role changed Admission class or byte bound", purpose, class)
		}
	}
	for _, purpose := range []ardp.Purpose{0, 2, 3, 5, 8, 255} {
		if _, err := AdmissionClass(purpose); err == nil {
			t.Fatal("unimplemented terminal accepted", purpose)
		}
	}
	if AdmissionWireBytes != 617 {
		t.Fatal("initial frame accounting changed", AdmissionWireBytes)
	}
}

func presentationTLS(t *testing.T) (*tls.Conn, *tls.Conn, func()) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(private) })
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	var expected [32]byte
	copy(expected[:], public)
	local, remote := net.Pipe()
	interrupt := func() { _ = local.Close(); _ = remote.Close() }
	t.Cleanup(interrupt)
	deadline := now.Add(5 * time.Second)
	if err := local.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if err := remote.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	client := tls.Client(local, transport.RoleClientTLS(expected))
	server := tls.Server(remote, transport.RoleServerTLS(tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private}))
	accepted := make(chan error, 1)
	go func() { accepted <- server.HandshakeContext(t.Context()) }()
	if err := client.HandshakeContext(t.Context()); err != nil {
		interrupt()
		<-accepted
		t.Fatal(err)
	}
	if err := <-accepted; err != nil {
		t.Fatal(err)
	}
	return client, server, interrupt
}
