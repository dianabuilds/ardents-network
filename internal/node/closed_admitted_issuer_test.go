//go:build linux

package node

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// Public State and offline authority are fixtures. Issuance, blinding, three
// production listeners, forwarding admission, nested TLS and receiver spends
// are real, including the retained Route terminal-channel consumer.
func TestClosedIssuerAdmittedOperationAfterTwoBootstrapBatches(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newClosedBootstrapNetwork(t, carrier)
			profile := fixture.view.Profile
			holderPublic, holder, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(holder)
			permission := credential.Permission{NetworkID: profile.NetworkID, IssuerNodeID: profile.IssuerNodeID,
				DutyGeneration: profile.IssuerDutyGeneration, PermissionID: [32]byte{91}, NotBefore: profile.NotBefore,
				NotAfter: profile.NotAfter, Maxima: [3]uint32{4, 3, 0}, Signature: [64]byte{1}}
			copy(permission.HolderKey[:], holderPublic)
			raw, err := credential.EncodePermission(permission)
			if err != nil {
				t.Fatal(err)
			}
			transcript := append([]byte("ardents-issuance-permission-v1\x00"), raw[:len(raw)-64]...)
			copy(permission.Signature[:], ed25519.Sign(fixture.authority, transcript))
			challenge := func(index int, class uint8) credential.ClosedTokenContext {
				receiver := fixture.view.Nodes[index]
				return credential.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest,
					IssuerNodeID: profile.IssuerNodeID, ReceiverNodeID: receiver.NodeID,
					ReceiverDutyGeneration: receiver.DutyGeneration, Class: class, WindowStart: profile.NotBefore}
			}
			prepare := func(contexts ...credential.ClosedTokenContext) *credential.PendingClosedTokenBatch {
				t.Helper()
				pending, err := credential.PrepareClosedTokenBatch(credential.ClosedTokenBatchConfig{Profile: profile,
					Contexts: contexts, Permission: permission, HolderKey: holder, Now: time.Now().UTC()})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(pending.Discard)
				return pending
			}
			bootstrap := func(pending *credential.PendingClosedTokenBatch) [][]byte {
				t.Helper()
				result, err := route.ExchangeClosedBootstrap(t.Context(), fixture, fixture.selection, pending.Request())
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
			third, err := route.ExchangeClosedBootstrap(t.Context(), fixture, fixture.selection, pending.Request())
			if err != nil {
				t.Fatal(err)
			}
			exhausted, err := route.DecodeClosedIssuanceResult(third.Body, third.Nonce)
			if err != nil || exhausted.Status != 2 {
				t.Fatalf("third bootstrap was not exhausted: %d / %v", exhausted.Status, err)
			}
			prefix, err := route.OpenClosedSourcePrefix(t.Context(), fixture, fixture.selection,
				func(hello route.ClosedHello, class uint8) ([]byte, error) {
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
			exchange := func(token []byte) (route.ClosedIssuanceExchangeResult, error) {
				return prefix.ExchangeIssuer(t.Context(), func(hello route.ClosedHello, class uint8) ([]byte, error) {
					if hello.RecipientNodeID != profile.IssuerNodeID || hello.Purpose != route.ClosedPurposeIssuer || class != 1 {
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
			firstResult, err := route.DecodeClosedIssuanceResult(first.Body, first.Nonce)
			if err != nil {
				t.Fatal(err)
			}
			retryResult, err := route.DecodeClosedIssuanceResult(retried.Body, retried.Nonce)
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
					if err := credential.VerifyClosedToken(challenge(0, 2), key.SPKI[:], tokens[0]); err != nil {
						t.Fatal(err)
					}
					return
				}
			}
			t.Fatal("fixture class key missing")
		})
	}
}
