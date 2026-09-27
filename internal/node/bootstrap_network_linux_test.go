//go:build linux

package node

import (
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"

	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestClosedBootstrapClientIssuesThroughEntryInteriorAndIssuer(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newClosedBootstrapNetwork(t, carrier)
			pending, challenge, spki := fixture.batch(t, 32)
			defer pending.Discard()
			for attempt := 0; attempt < 2; attempt++ {
				// The same request exercises durable exact retry. Permission has budget
				// for only one batch, so a second debit cannot also return 32 signatures.
				result, err := client.ExchangeClosedBootstrap(t.Context(), fixture, fixture.selection, pending.Request())
				if err != nil {
					t.Fatalf("network exchange %d: %v", attempt, err)
				}
				decoded, err := terminal.DecodeIssuanceResult(result.Body, result.Nonce)
				if err != nil || decoded.Status != 0 {
					t.Fatalf("issuer result: %d %v", decoded.Status, err)
				}
				if attempt == 0 {
					continue
				}
				tokens, err := pending.FinalizeTerminalOperation(result.Nonce, result.Body)
				if err != nil || len(tokens) != 32 {
					t.Fatalf("finalize: %d %v", len(tokens), err)
				}
				for _, token := range tokens {
					if err := credential.VerifyClosedToken(challenge, spki[:], token); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestClosedSourcePrefixConsumesGenuineForwardingTokens(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newClosedBootstrapNetwork(t, carrier)
			profile := fixture.view.Profile
			var challenges []credential.ClosedTokenContext
			for index, receiver := range [][32]byte{fixture.selection.EntryNodeID, fixture.selection.InteriorNodeID} {
				challenges = append(challenges, credential.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest,
					IssuerNodeID: profile.IssuerNodeID, ReceiverNodeID: receiver, ReceiverDutyGeneration: uint64(index + 1), Class: 2, WindowStart: profile.NotBefore})
			}
			// One permission and one genuine blind batch fund both forwarding
			// receivers; their challenge bindings remain private to this client.
			pending, key := fixture.batchForChallenges(t, challenges, 80)
			result, err := client.ExchangeClosedBootstrap(t.Context(), fixture, fixture.selection, pending.Request())
			if err != nil {
				pending.Discard()
				t.Fatal(err)
			}
			batch, err := pending.FinalizeTerminalOperation(result.Nonce, result.Body)
			if err != nil || len(batch) != 2 {
				t.Fatalf("issuance: %v", err)
			}
			tokens := make(map[[32]byte][]byte)
			for index, challenge := range challenges {
				if err := credential.VerifyClosedToken(challenge, key[:], batch[index]); err != nil {
					t.Fatal(err)
				}
				if err := credential.VerifyClosedToken(challenges[1-index], key[:], batch[index]); err == nil {
					t.Fatal("token crossed receiving challenges")
				}
				tokens[challenge.ReceiverNodeID] = batch[index]
			}
			presented := 0
			prefix, err := client.OpenClosedSourcePrefix(t.Context(), fixture, fixture.selection, func(hello ardp.Hello, class uint8) ([]byte, error) {
				token := tokens[hello.RecipientNodeID]
				if class != 2 || len(token) != 354 {
					return nil, errors.New("token already consumed or wrong receiver")
				}
				delete(tokens, hello.RecipientNodeID)
				presented++
				return token, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if presented != 2 || len(tokens) != 0 {
				t.Fatal("prefix did not consume both admissions")
			}
			if err := prefix.Close(); err != nil {
				t.Fatal(err)
			}
			if err := prefix.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
