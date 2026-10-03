//go:build linux

package node

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	admissionissuer "github.com/dianabuilds/ardents-network/internal/admission/issuer"
	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"
	"strings"
	"sync"
	"testing"
	"time"

	admissiongrammar "github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

func TestRunServesClosedIssuerThenDrainsOnClosedProfileSuccessor(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	until := now.Truncate(time.Hour).Add(time.Hour)
	certificate, public := nodeCertificate(t, 211, "closed-issuer")
	network, issuerID := [32]byte{41}, [32]byte{42}
	generation, digest := sha256.Sum256([]byte("closed node generation")), sha256.Sum256([]byte("closed node digest"))
	root := closedIssuerFixtureRoot(t)
	receipt, err := admissionissuer.InitializeClosedIssuerRoot(admissionissuer.ClosedIssuerRootConfig{Root: root, NetworkID: network, NodeID: issuerID,
		IdentityKey: certificate.PrivateKey.(ed25519.PrivateKey), NotBefore: now.Truncate(time.Hour), NotAfter: now.Truncate(time.Hour).Add(time.Hour), Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	issuerProfile, err := admissiongrammar.DecodeClosedIssuerProfile(receipt.Profile, ed25519.PublicKey(public[:]))
	if err != nil {
		t.Fatal(err)
	}
	profile := state.ClosedProfileView{NetworkID: network, StateGeneration: generation, StateDigest: digest, Digest: sha256.Sum256([]byte("closed profile")),
		IssuanceAuthorityKey: [32]byte{43}, IssuerNodeID: issuerID, IssuerDutyGeneration: 8, Epoch: 4,
		NotBefore: now.Truncate(time.Hour), NotAfter: now.Truncate(time.Hour).Add(time.Hour), TokenKeyCount: uint8(len(issuerProfile.Keys))}
	for index, key := range issuerProfile.Keys {
		profile.TokenKeys[index].WindowStart, profile.TokenKeys[index].Class = key.WindowStart, uint8(key.Class)
		copy(profile.TokenKeys[index].SPKI[:], key.SPKI)
	}
	recordDigest := [32]byte{45}
	snapshot := state.NodeDuty{Generation: hex.EncodeToString(generation[:]), NetworkID: network, Epoch: profile.Epoch, Digest: digest,
		EpochValidFrom: profile.NotBefore, ValidUntil: until, Profile: carrier.ClosedRouteProfile, Fresh: true, RecordPresent: true,
		NodeID: issuerID, NodePublicKey: public, RecordGeneration: profile.IssuerDutyGeneration, RecordValidFrom: now.Add(-time.Second), RecordValidUntil: until,
		DeclaredFamily: "closed-issuer-family", ProbeEndpoint: reserveAddress(t), CarrierProfile: string(carrier.ClosedCarrierTCP), Assignment: "rendezvous", AssignmentDigest: [32]byte{44}}
	var lock sync.RWMutex
	events := make(chan Event, 16)
	config := Config{HostingRoot: hostingFixtureRoot(t), NetworkID: network, NodeID: issuerID, IdentityKey: certificate.PrivateKey.(ed25519.PrivateKey),
		Current:              func() (state.NodeDuty, error) { lock.RLock(); defer lock.RUnlock(); return snapshot, nil },
		CurrentClosedProfile: func() (state.ClosedProfileView, bool) { return profile, true },
		CurrentClosedRoute: func() (state.ClosedRouteView, error) {
			view := state.ClosedRouteView{Profile: profile, NodeCount: 1}
			view.Nodes[0] = state.ClosedRouteNodeView{NodeID: issuerID, RecordDigest: recordDigest, RoleDomain: 2, Subrole: 6, DutyGeneration: profile.IssuerDutyGeneration}
			return view, nil
		},
		ClosedIssuer: ClosedIssuerProfile{Root: root, AdmissionRoot: t.TempDir(), Certificate: certificate, ConnectionLimit: 1, DrainTimeout: time.Second},
		PollInterval: 10 * time.Millisecond, Quarantine: time.Millisecond, LocalRoleStateRoot: localRoleStateRoot(t), CheckPlacement: func() error { return nil },
		Emit: func(_ context.Context, event Event) error { events <- event; return nil }}
	resolved, err := resolveConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if admission := assessAdmission(resolved, snapshot); admission.kind != admissionReady {
		t.Fatalf("closed issuer admission = %+v", admission)
	}
	results := make(chan Result, 1)
	runErrors := make(chan error, 1)
	go func() { result, runErr := Run(context.Background(), config); results <- result; runErrors <- runErr }()
	waitForStateEvent(t, events, "READY")
	lock.Lock()
	snapshot.Generation = strings.Repeat("b", 64)
	lock.Unlock()
	waitForStateEvent(t, events, "DRAINING")
	select {
	case result := <-results:
		if result.State != "WITHDRAWN" {
			t.Fatalf("closed successor result = %+v", result)
		}
	case <-time.After(testLifecycleWait):
		t.Fatal("closed profile successor did not drain issuer")
	}
	if err := <-runErrors; err != nil {
		t.Fatal(err)
	}
}

// Public State and offline authority are fixtures. Issuance, blinding, three
// production listeners, forwarding admission, nested TLS and receiver spends
// are real, including the retained Route terminal-channel consumer.
func TestClosedIssuerAdmittedOperationAfterTwoBootstrapBatches(t *testing.T) {
	for _, carrier := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newClosedBootstrapNetwork(t, carrier)
			profile := fixture.view.Profile
			holderPublic, holder, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(holder)
			permission := admissiongrammar.Permission{NetworkID: profile.NetworkID, IssuerNodeID: profile.IssuerNodeID,
				DutyGeneration: profile.IssuerDutyGeneration, PermissionID: [32]byte{91}, NotBefore: profile.NotBefore,
				NotAfter: profile.NotAfter, Maxima: [3]uint32{4, 3, 0}, Signature: [64]byte{1}}
			copy(permission.HolderKey[:], holderPublic)
			raw, err := admissiongrammar.EncodePermission(permission)
			if err != nil {
				t.Fatal(err)
			}
			transcript := append([]byte("ardents-issuance-permission-v1\x00"), raw[:len(raw)-64]...)
			copy(permission.Signature[:], ed25519.Sign(fixture.authority, transcript))
			challenge := func(index int, class uint8) admissiontoken.ClosedTokenContext {
				receiver := fixture.view.Nodes[index]
				return admissiontoken.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest,
					IssuerNodeID: profile.IssuerNodeID, ReceiverNodeID: receiver.NodeID,
					ReceiverDutyGeneration: receiver.DutyGeneration, Class: class, WindowStart: profile.NotBefore}
			}
			prepare := func(contexts ...admissiontoken.ClosedTokenContext) *admissiontoken.PendingClosedTokenBatch {
				t.Helper()
				pending, err := admissiontoken.PrepareClosedTokenBatch(admissiontoken.ClosedTokenBatchConfig{Profile: profile,
					Contexts: contexts, Permission: permission, HolderKey: holder, Now: time.Now().UTC()})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(pending.Discard)
				return pending
			}
			bootstrap := func(pending *admissiontoken.PendingClosedTokenBatch) [][]byte {
				t.Helper()
				result, err := client.ExchangeClosedBootstrap(t.Context(), fixture, fixture.selection, pending.Request())
				if err != nil {
					t.Fatal(err)
				}
				tokens, err := pending.FinalizeTerminalOperation(result.Nonce, result.Body)
				if err != nil {
					t.Fatal(err)
				}
				return tokens
			}
			forward := bootstrap(prepare(challenge(0, 2), challenge(1, 2)))
			control := bootstrap(prepare(challenge(2, 1), challenge(2, 1), challenge(2, 1)))
			pending := prepare(challenge(0, 2))
			third, err := client.ExchangeClosedBootstrap(t.Context(), fixture, fixture.selection, pending.Request())
			if err != nil {
				t.Fatal(err)
			}
			exhausted, err := terminal.DecodeIssuanceResult(third.Body, third.Nonce)
			if err != nil || exhausted.Status != 2 {
				t.Fatalf("third bootstrap was not exhausted: %d / %v", exhausted.Status, err)
			}
			prefix, err := client.OpenClosedSourcePrefix(t.Context(), fixture, fixture.selection,
				func(hello ardp.Hello, class uint8) ([]byte, error) {
					for index := 0; index < 2; index++ {
						if hello.RecipientNodeID == fixture.view.Nodes[index].NodeID && class == 2 && forward[index] != nil {
							token := forward[index]
							forward[index] = nil
							return token, nil
						}
					}
					return nil, errors.New("fixture forwarding stock unavailable")
				})
			if err != nil {
				t.Fatal(err)
			}
			defer prefix.Close()
			exchange := func(token []byte) (client.ClosedIssuanceExchangeResult, error) {
				return prefix.ExchangeIssuer(t.Context(), func(hello ardp.Hello, class uint8) ([]byte, error) {
					if hello.RecipientNodeID != profile.IssuerNodeID || hello.Purpose != ardp.PurposeIssuer || class != 1 {
						return nil, errors.New("wrong receiving token challenge")
					}
					return bytes.Clone(token), nil
				}, pending.Request())
			}
			first, err := exchange(control[0])
			if err != nil {
				t.Fatal(err)
			}
			if _, err := exchange(control[0]); err == nil {
				t.Fatal("issuer accepted a spent Control token on another TLS channel")
			}
			retried, err := exchange(control[1])
			if err != nil {
				t.Fatal(err)
			}
			firstResult, err := terminal.DecodeIssuanceResult(first.Body, first.Nonce)
			if err != nil {
				t.Fatal(err)
			}
			retryResult, err := terminal.DecodeIssuanceResult(retried.Body, retried.Nonce)
			if err != nil || !bytes.Equal(firstResult.Payload, retryResult.Payload) {
				t.Fatalf("exact batch retry on fresh admission differs: %v", err)
			}
			invalid := bytes.Clone(control[2])
			invalid[len(invalid)-1] ^= 1
			if _, err := exchange(invalid); err == nil {
				t.Fatal("issuer admitted invalid signature")
			}
			if _, err := exchange(control[2]); err != nil {
				t.Fatalf("invalid signature refusal spent genuine Control token: %v", err)
			}
			tokens, err := pending.FinalizeTerminalOperation(first.Nonce, first.Body)
			if err != nil || len(tokens) != 1 {
				t.Fatalf("normal admitted result did not finalize: %v", err)
			}
			for _, key := range profile.TokenKeys[:profile.TokenKeyCount] {
				if key.Class == 2 && key.WindowStart == profile.NotBefore {
					if err := admissiontoken.VerifyClosedToken(challenge(0, 2), key.SPKI[:], tokens[0]); err != nil {
						t.Fatal(err)
					}
					return
				}
			}
			t.Fatal("fixture class key missing")
		})
	}
}
