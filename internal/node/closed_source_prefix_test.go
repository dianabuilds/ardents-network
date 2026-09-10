//go:build linux

package node

import (
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// Real source bootstrap obtains both receiving tokens; the subsequent prefix
// has fresh TLS/admission at Entry and Interior. Public State/offline allocation
// remain explicit fixtures, not enrollment or Endpoint journal qualification.
func TestClosedSourcePrefixConsumesGenuineForwardingTokens(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
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
			result, err := route.ExchangeClosedBootstrap(t.Context(), fixture, fixture.selection, pending.Request())
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
			prefix, err := route.OpenClosedSourcePrefix(t.Context(), fixture, fixture.selection, func(hello route.ClosedHello, class uint8) ([]byte, error) {
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
