//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// This proves prepared role-3 selection/reservation -> actual Source-bound
// forwarding on both Carriers. It supplies no qualified Publisher, capsule,
// accepted nonce or successful delivery RESULT.
func TestPublicationPreparedResponderOpensWithOriginalSourceBothCarriers(t *testing.T) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := newJoinRouteFixture(t, carrier)
			holder := joinRouteStock(t, f, admission.AllocationPublisher)
			startJoinRouteReceivers(t, f, sockets, certificates)
			root := t.TempDir()
			plan := routePrefixPlan{Deadline: time.Now().Add(40 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			installation, err := selection.OpenInstallation(selection.InstallationConfig{EntryRoot: filepath.Join(root, "entry"), Current: f.authority.current})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := installation.Close(); err != nil {
					t.Error(err)
				}
			}()
			sourceSelection, err := installation.Borrow(selection.RoleConfig{InteriorRoot: filepath.Join(root, "source"), Domain: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := sourceSelection.Close(); err != nil {
					t.Error(err)
				}
			}()
			responderSelection, err := installation.Borrow(selection.RoleConfig{InteriorRoot: filepath.Join(root, "responder"), Domain: 3})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := responderSelection.Close(); err != nil {
					t.Error(err)
				}
			}()
			budget, err := hosting.Open(routeProcessBudget(t))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := budget.Close(); err != nil {
					t.Error(err)
				}
			}()
			source, _, err := openPublicationPrefix(t.Context(), plan, sourceSelection, budget, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := source.Close(); err != nil {
					t.Error(err)
				}
			}()
			prepared, err := preparePublicationPrefix(t.Context(), plan, responderSelection, budget, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := prepared.Release(); err != nil {
					t.Error(err)
				}
			}()
			responder, err := prefix.OpenResponder(t.Context(), source, prepared)
			if err != nil {
				t.Fatal(err)
			}
			if err := responder.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRouteGenuinePairedJoinBothCarriers(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			f, sockets, certificates := newJoinRouteFixture(t, profile)
			userStock := joinRouteStock(t, f, admission.AllocationUser)
			publisherStock := joinRouteStock(t, f, admission.AllocationPublisher)
			receivers, budgets := startJoinRouteReceivers(t, f, sockets, certificates)
			plan := func(domain uint8) routePrefixPlan {
				root := t.TempDir()
				return routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), SourceInteriorRoot: filepath.Join(root, "source-interior"), HostingRoot: routeProcessBudget(t), Domain: domain, Deadline: time.Now().Add(60 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			}
			userPlan, publisherPlan := plan(1), plan(3)
			userContext, err := newRouteJoinContext(t.Context(), userPlan, f.authority, userStock)
			if err != nil {
				t.Fatal(err)
			}
			defer userContext.close()
			user, err := userContext.open(t.Context())
			if err != nil {
				t.Fatal("User Source", err)
			}
			defer user.close()
			publisherContext, err := newRouteJoinContext(t.Context(), publisherPlan, f.authority, publisherStock)
			if err != nil {
				t.Fatal(err)
			}
			defer publisherContext.close()
			publisher, err := publisherContext.open(t.Context())
			if err != nil {
				t.Fatal("same-installation Source and Responder", err)
			}
			defer publisher.close()
			recipient, err := user.recipient(0)
			if err != nil {
				t.Fatal(err)
			}
			if recipient.Node != [32]byte{20} && recipient.Node != [32]byte{21} {
				t.Fatal("not a genuine eligible Rendezvous")
			}
			intent := routeJoinIntent{Node: recipient.Node, Generation: recipient.Generation, Deadline: minRouteDeadline(userPlan.Deadline, publisherPlan.Deadline, recipient.NotAfter), SetupDeadline: time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)}
			if _, err := rand.Read(intent.Secret[:]); err != nil {
				t.Fatal(err)
			}
			if _, err := rand.Read(intent.Context[:]); err != nil {
				t.Fatal(err)
			}
			type outcome struct {
				side   int
				stream net.Conn
				err    error
			}
			opened := make(chan outcome, 2)
			go func() { stream, err := user.join(t.Context(), intent); opened <- outcome{0, stream, err} }()
			go func() { stream, err := publisher.join(t.Context(), intent); opened <- outcome{1, stream, err} }()
			var streams [2]net.Conn
			var failure error
			for range 2 {
				result := <-opened
				streams[result.side] = result.stream
				failure = errors.Join(failure, result.err)
			}
			defer func() {
				for _, stream := range streams {
					if stream != nil {
						_ = stream.Close()
					}
				}
			}()
			if failure != nil {
				t.Fatal("paired real admitted JOIN", failure)
			}
			joinedFixtureTLS(t, streams)
			for side, stream := range streams {
				if err := stream.Close(); err != nil {
					t.Errorf("joined stream close side %d: %v", side, err)
				}
			}
			select {
			case <-publisher.done:
				t.Error("Responder readiness retired before explicit prefix close")
			default:
			}
			userClose, publisherClose := user.close(), publisher.close()
			if userClose != nil || publisherClose != nil {
				t.Fatal("joined original prefix resources: User", userClose, "Publisher", publisherClose)
			}
			if err := errors.Join(userContext.close(), publisherContext.close()); err != nil {
				t.Fatal("joined Route contexts", err)
			}
			for _, receiver := range receivers {
				if err := receiver.Close(); err != nil {
					t.Fatal("receiving joined outcome", err)
				}
			}
			for _, budget := range budgets {
				observation, err := budget.Observe(t.Context())
				if err != nil || observation.ReservedBytes != 0 {
					t.Fatal("receiving reservation did not return after join", observation.ReservedBytes, err)
				}
			}
			for _, root := range []string{userPlan.HostingRoot, publisherPlan.HostingRoot} {
				budget, err := hosting.Open(root)
				if err != nil {
					t.Fatal("installation root not released", err)
				}
				observation, err := budget.Observe(t.Context())
				closeErr := budget.Close()
				if err != nil || closeErr != nil || observation.ReservedBytes != 0 {
					t.Fatal("installation reservation not joined", err, closeErr, observation.ReservedBytes)
				}
			}
		})
	}
}

// These ephemeral mutually pinned fixture identities establish actual opaque
// encrypted traffic through Route, never Service Instance authentication.
func joinedFixtureTLS(t *testing.T, streams [2]net.Conn) {
	t.Helper()
	var certs [2]tls.Certificate
	var keys [2]ed25519.PublicKey
	for i := range certs {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { clear(private) })
		keys[i], certs[i] = public, routeTestCertificate(t, private)
	}
	pin := func(expected ed25519.PublicKey) func(tls.ConnectionState) error {
		return func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 1 {
				return errors.New("fixture peer absent")
			}
			key, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
			if !ok || !bytes.Equal(key, expected) {
				return errors.New("fixture peer changed")
			}
			return nil
		}
	}
	client := tls.Client(streams[0], &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certs[0]}, InsecureSkipVerify: true, VerifyConnection: pin(keys[1])})
	server := tls.Server(streams[1], &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certs[1]}, ClientAuth: tls.RequireAnyClientCert, VerifyConnection: pin(keys[0])})
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	if err := errors.Join(client.SetDeadline(time.Now().Add(15*time.Second)), server.SetDeadline(time.Now().Add(15*time.Second))); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.HandshakeContext(ctx) }()
	if err := errors.Join(client.HandshakeContext(ctx), <-done); err != nil {
		t.Fatal("opaque TLS through joined Route", err)
	}
	payload := bytes.Repeat([]byte{0x42}, 96<<10)
	go func() { _, err := client.Write(payload); done <- err }()
	got := make([]byte, len(payload))
	_, readErr := io.ReadFull(server, got)
	if err := errors.Join(readErr, <-done); err != nil || !bytes.Equal(got, payload) {
		t.Fatal("credit-controlled encrypted payload", err)
	}
	go func() { done <- server.CloseWrite() }()
	if err := errors.Join(client.CloseWrite(), <-done); err != nil {
		t.Fatal("fixture TLS terminal", err)
	}
	var one [1]byte
	if _, err := client.Read(one[:]); err != io.EOF {
		t.Fatal("fixture client terminal", err)
	}
	if _, err := server.Read(one[:]); err != io.EOF {
		t.Fatal("fixture server terminal", err)
	}
}
