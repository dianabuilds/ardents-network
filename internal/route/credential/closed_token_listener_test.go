//go:build linux

package credential

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"net"
	"os"
	"testing"
	"time"
)

func TestClosedTokenListenerServesOnlyDirectRoleBootstrap(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			issuer, profile, operation, now := closedTokenListenerIssuer(t)
			defer func() {
				if err := issuer.Close(); err != nil {
					t.Error(err)
				}
			}()
			before := observeClosedIssuer(t, issuer, "before actual TLS issuance")
			certificate, server := closedTokenListenerCertificate(t)
			endpoint := closedTokenListenerEndpoint(t, carrier)
			listener, err := StartClosedTokenListener(t.Context(), ClosedTokenListenerConfig{
				Issuer: issuer, CarrierProfile: carrier, Endpoint: endpoint, Certificate: certificate,
				ConnectionLimit: 1, Clock: func() time.Time { return now },
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = listener.Stop() })
			connection, err := route.OpenClosedRoleCarrier(t.Context(), route.ClosedRoleCarrierRequest{
				CarrierProfile: carrier, Endpoint: endpoint, ExpectedServer: server, Deadline: time.Now().Add(10 * time.Second),
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := route.ClosedRoleTLSExporter(connection); err != nil {
				t.Fatal(err)
			}
			observed := &issuerTLSObservationConn{Conn: connection}
			result := closedTokenListenerBootstrap(t, observed, profile, now, operation)
			if _, err := route.DecodeClosedIssuanceResult(result, [32]byte{71}); err != nil {
				t.Fatalf("listener result: %v", err)
			}
			_ = connection.Close()
			drain, cancel := contextWithDeadline(t, time.Second)
			defer cancel()
			if err := listener.Drain(drain); err != nil {
				t.Fatal(err)
			}
			if err := <-listener.Done(); err != nil {
				t.Fatal(err)
			}
			checkIssuerTLSObservation(t, issuer, listener, carrier, server, observed, operation, result, before)
		})
	}
}

func closedTokenListenerIssuer(t *testing.T) (*ClosedTokenIssuer, state.ClosedProfileView, []byte, time.Time) {
	t.Helper()
	window := time.Unix(1_800_100_000, 0).UTC().Truncate(time.Hour)
	now := window.Add(time.Minute)
	nodePublic, nodePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	network, issuerNode := sha256.Sum256([]byte("listener network")), sha256.Sum256([]byte("listener node"))
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	receipt, err := InitializeClosedIssuerRoot(ClosedIssuerRootConfig{Root: root, NetworkID: network, NodeID: issuerNode, IdentityKey: nodePrivate,
		NotBefore: window, NotAfter: window.Add(time.Hour), Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	issuerProfile, err := DecodeClosedIssuerProfile(receipt.Profile, nodePublic)
	if err != nil {
		t.Fatal(err)
	}
	authority := ed25519.NewKeyFromSeed(bytesForClosedTokenBatch(72))
	profile := state.ClosedProfileView{NetworkID: network, StateGeneration: sha256.Sum256([]byte("listener State generation")), StateDigest: sha256.Sum256([]byte("listener State digest")),
		Digest: sha256.Sum256([]byte("listener accepted State profile")), IssuerNodeID: issuerNode, IssuerDutyGeneration: 4,
		NotBefore: window, NotAfter: window.Add(time.Hour), TokenKeyCount: uint8(len(issuerProfile.Keys))}
	copy(profile.IssuanceAuthorityKey[:], authority.Public().(ed25519.PublicKey))
	for index, key := range issuerProfile.Keys {
		profile.TokenKeys[index].WindowStart, profile.TokenKeys[index].Class = key.WindowStart, uint8(key.Class)
		copy(profile.TokenKeys[index].SPKI[:], key.SPKI)
	}
	holder := ed25519.NewKeyFromSeed(bytesForClosedTokenBatch(73))
	permission := Permission{NetworkID: network, IssuerNodeID: issuerNode, DutyGeneration: profile.IssuerDutyGeneration,
		PermissionID: sha256.Sum256([]byte("listener permission")), NotBefore: window, NotAfter: window.Add(time.Hour), Maxima: [3]uint32{2, 0, 0}}
	copy(permission.HolderKey[:], holder.Public().(ed25519.PublicKey))
	copy(permission.Signature[:], ed25519.Sign(authority, permissionTranscript(permission)))
	context := ClosedTokenContext{NetworkID: network, ProfileDigest: profile.Digest, ReceiverNodeID: sha256.Sum256([]byte("listener receiver")),
		IssuerNodeID: issuerNode, ReceiverDutyGeneration: 6, Class: 1, WindowStart: window}
	pending, err := PrepareClosedTokenBatch(ClosedTokenBatchConfig{Profile: profile, Contexts: []ClosedTokenContext{context}, Permission: permission, HolderKey: holder, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pending.Discard)
	operation, err := route.EncodeClosedIssuanceRequest([32]byte{71}, pending.Request())
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := OpenClosedTokenIssuer(ClosedTokenIssuerConfig{Root: root, NetworkID: network, CurrentProfile: func() (state.ClosedProfileView, bool) { return profile, true }, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return issuer, profile, operation, now
}

func closedTokenListenerBootstrap(t *testing.T, connection net.Conn, profile state.ClosedProfileView, now time.Time, operation []byte) []byte {
	t.Helper()
	hello := route.ClosedHello{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.StateDigest,
		ProfileDigest: profile.Digest, RecipientNodeID: profile.IssuerNodeID, RecipientDutyGeneration: profile.IssuerDutyGeneration,
		Purpose: route.ClosedPurposeIssuer, ChannelNonce: [32]byte{74}, Deadline: now.Add(10 * time.Second)}
	body, err := route.EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(connection, route.ClosedLaneFrame{Kind: 1, Lane: 0, Body: body}); err != nil {
		t.Fatal(err)
	}
	accepted, err := route.ReadClosedLaneFrame(connection)
	if err != nil {
		t.Fatal(err)
	}
	if status, _, err := route.DecodeClosedAcceptFrame(accepted); err != nil || status != 0 {
		t.Fatalf("listener accept = %d / %v", status, err)
	}
	if err := route.WriteClosedLaneFrame(connection, route.ClosedLaneFrame{Kind: 3, Lane: 0, Body: route.EncodeClosedBootstrap(true)}); err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(connection, route.ClosedLaneFrame{Kind: 10, Lane: 0, Body: operation}); err != nil {
		t.Fatal(err)
	}
	result, err := route.ReadClosedLaneFrame(connection)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != 11 || result.Lane != 0 || len(result.Body) != 16<<10 {
		t.Fatalf("listener result frame = %+v", result)
	}
	return result.Body
}

func contextWithDeadline(t *testing.T, timeout time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(t.Context(), timeout)
}
