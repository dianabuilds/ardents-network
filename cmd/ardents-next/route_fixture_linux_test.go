//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/route/carrier"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func routeTestCertificate(t *testing.T, key ed25519.PrivateKey) tls.Certificate {
	t.Helper()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: key}
}

func newRouteFixture(t *testing.T, profile carrier.CarrierProfile) (*networkAdmissionFixture, map[[32]byte]func(), map[[32]byte]tls.Certificate) {
	return newRoleRouteFixture(t, profile, 3, false)
}

func newRoleRouteFixture(t *testing.T, profile carrier.CarrierProfile, domain byte, introduction bool) (*networkAdmissionFixture, map[[32]byte]func(), map[[32]byte]tls.Certificate) {
	t.Helper()
	reservations := make(map[[32]byte]func())
	certificates := make(map[[32]byte]tls.Certificate)
	f := newNetworkAdmissionFixture(t, func(f *networkAdmissionFixture) {
		count := 4
		if introduction {
			count++
		}
		for i := 0; i < count; i++ {
			id := [32]byte{byte(12 + i)}
			var key ed25519.PrivateKey
			if i == 0 {
				key = f.spec.Nodes[1].PrivateKey
			} else {
				_, key, _ = ed25519.GenerateKey(rand.Reader)
				t.Cleanup(func() { clear(key) })
			}
			var address string
			if profile == carrier.ClosedCarrierTCP {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				address = listener.Addr().String()
				reservations[id] = func() { _ = listener.Close() }
			} else {
				socket, err := net.ListenPacket("udp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				address = socket.LocalAddr().String()
				reservations[id] = func() { _ = socket.Close() }
			}
			t.Cleanup(reservations[id])
			subrole := byte(1)
			if i >= 2 {
				subrole = 2
			}
			if i == 4 {
				subrole = 3
			}
			node := networkfixture.ClosedNode{RecordSpec: networkfixture.RecordSpec{NodeID: id, Generation: 9, ValidFrom: f.spec.NotBefore, ValidUntil: f.spec.NotAfter, Endpoint: address, Carrier: string(profile), Capability: 2, Capacity: 1, PrivateKey: key}, RoleDomain: domain, Subrole: subrole}
			if i == 0 {
				f.spec.Nodes[1] = node
			} else {
				f.spec.Nodes = append(f.spec.Nodes, node)
			}
			certificates[id] = routeTestCertificate(t, key)
		}
	})
	return f, reservations, certificates
}

// routeStock uses genuine allocation/signing, blind issuance and Stock. All
// receiving choices are signed members, and no token is taken before TLS HELLO.
func routeStock(t *testing.T, f *networkAdmissionFixture) *stock.Owner {
	return routeRoleStock(t, f, false)
}

func routeRoleStock(t *testing.T, f *networkAdmissionFixture, introduction bool, refill ...bool) *stock.Owner {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	role := admission.AllocationUser
	maxima := [3]uint32{0, 4, 0}
	if len(refill) != 0 && refill[0] {
		maxima[1] = 8
	}
	if introduction {
		role = admission.AllocationPublisher
		maxima[2] = 1
	}
	holder, err := stock.Open(root, role, f.authority.observe)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := holder.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, digest, err := holder.Request(maxima)
	if err != nil {
		t.Fatal(err)
	}
	request, err := allocation.Prepare(raw, f.profile.NetworkID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	decision, err := request.Decide(nil, f.profile.IssuanceAuthorityKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(t.TempDir(), "allocation")
	if err := os.WriteFile(journal, decision.Journal(), 0600); err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(committed, decision.Journal()) {
		t.Fatal("allocation durable readback", err)
	}
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(f.spec.Authority, admission.PermissionTranscript(permission)))
	encoded, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Import(digest, encoded); err != nil {
		t.Fatal(err)
	}
	var challenges []token.ClosedTokenContext
	for i := byte(12); i < 16; i++ {
		challenges = append(challenges, token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: [32]byte{i}, ReceiverDutyGeneration: 9, Class: 2, WindowStart: time.Now().UTC().Truncate(time.Hour)})
		if len(refill) != 0 && refill[0] {
			challenges = append(challenges, challenges[len(challenges)-1])
		}
	}
	batches := [][]token.ClosedTokenContext{challenges}
	if introduction {
		batches = append(batches, []token.ClosedTokenContext{{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: [32]byte{16}, ReceiverDutyGeneration: 9, Class: 3, WindowStart: time.Now().UTC().Truncate(time.Hour)}})
	}
	for index, contexts := range batches {
		attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: contexts, Selection: stock.ExchangeBinding{ID: [32]byte{byte(31 + index)}, ProfileDigest: f.profile.Digest}, Bootstrap: true, Deadline: f.profile.NotAfter})
		if err != nil {
			t.Fatal(err)
		}
		batch, _, err := attempt.Request()
		if err != nil {
			t.Fatal(err)
		}
		issued := issuer.IssueCurrent(t.Context(), f.plan, batch, quota.Bootstrap, func() (admission.AuthorityFacts, time.Time, error) {
			return f.authority.issuer(f.plan.KeyBinding.Signer)
		})
		if issued.Outcome != "issued-offline" {
			t.Fatal(issued)
		}
		if err := attempt.Complete(issued.Response, nil); err != nil {
			t.Fatal(err)
		}
	}
	return holder
}

// Permission/stock are volatile and must be created inside the actual holder
// process. Only its irreversible presentation history survives reopening.
func routeConsoleRoleStock(t *testing.T, f *networkAdmissionFixture, send func(any) localAdmissionReply, introduction bool, refill ...bool) {
	t.Helper()
	maxima := [3]uint32{0, 4, 0}
	if len(refill) != 0 && refill[0] {
		maxima[1] = 8
	}
	if introduction {
		maxima[2] = 1
	}
	request := send(holderCommand{Operation: "request", Maxima: maxima})
	if request.Outcome != "completed" {
		t.Fatal("holder request", request.Outcome)
	}
	prepared, err := allocation.Prepare(request.Request, f.profile.NetworkID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	decision, err := prepared.Decide(nil, f.profile.IssuanceAuthorityKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(t.TempDir(), "allocation")
	if err := os.WriteFile(journal, decision.Journal(), 0600); err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(committed, decision.Journal()) {
		t.Fatal("durable allocation", err)
	}
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(f.spec.Authority, admission.PermissionTranscript(permission)))
	raw, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	if reply := send(holderCommand{Operation: "import", Digest: request.Digest, Payload: raw}); reply.Outcome != "completed" {
		t.Fatal("holder import", reply.Outcome)
	}
	var challenges []token.ClosedTokenContext
	for i := byte(12); i < 16; i++ {
		challenges = append(challenges, token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: [32]byte{i}, ReceiverDutyGeneration: 9, Class: 2, WindowStart: time.Now().UTC().Truncate(time.Hour)})
		if len(refill) != 0 && refill[0] {
			challenges = append(challenges, challenges[len(challenges)-1])
		}
	}
	batches := [][]token.ClosedTokenContext{challenges}
	if introduction {
		batches = append(batches, []token.ClosedTokenContext{{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: [32]byte{16}, ReceiverDutyGeneration: 9, Class: 3, WindowStart: time.Now().UTC().Truncate(time.Hour)}})
	}
	for index, contexts := range batches {
		intent := stock.IssuanceIntent{Challenges: contexts, Selection: stock.ExchangeBinding{ID: [32]byte{byte(31 + index)}, ProfileDigest: f.profile.Digest}, Bootstrap: true, Deadline: f.profile.NotAfter}
		batch := send(holderCommand{Operation: "begin", Intent: intent})
		if batch.Outcome != "completed" {
			t.Fatal("holder begin", batch.Outcome)
		}
		issued := issuer.IssueCurrent(t.Context(), f.plan, batch.Request, quota.Bootstrap, func() (admission.AuthorityFacts, time.Time, error) {
			return f.authority.issuer(f.plan.KeyBinding.Signer)
		})
		if issued.Outcome != "issued-offline" {
			t.Fatal(issued.Outcome)
		}
		if reply := send(holderCommand{Operation: "complete", Payload: issued.Response}); reply.Outcome != "completed" {
			t.Fatal("holder complete", reply.Outcome)
		}
	}
}
