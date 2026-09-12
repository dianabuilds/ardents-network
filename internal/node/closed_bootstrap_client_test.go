//go:build linux

package node

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestClosedBootstrapClientIssuesThroughEntryInteriorAndIssuer(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newClosedBootstrapNetwork(t, carrier)
			pending, challenge, spki := fixture.batch(t, 32)
			defer pending.Discard()
			for attempt := 0; attempt < 2; attempt++ {
				// The same request exercises durable exact retry. Permission has budget
				// for only one batch, so a second debit cannot also return 32 signatures.
				result, err := route.ExchangeClosedBootstrap(t.Context(), fixture, fixture.selection, pending.Request())
				if err != nil {
					t.Fatalf("network exchange %d: %v", attempt, err)
				}
				decoded, err := route.DecodeClosedIssuanceResult(result.Body, result.Nonce)
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
