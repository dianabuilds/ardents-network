//go:build linux

package node

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestClosedBootstrapClientClassifiesUnreachableEntryCarrier(t *testing.T) {
	fixture := newClosedBootstrapNetwork(t, route.ClosedCarrierTCP)
	pending, _, _ := fixture.batch(t, 1)
	defer pending.Discard()
	fixture.snapshot.Candidates[0].Endpoint = reserveClosedBootstrapAddress(t, route.ClosedCarrierTCP)
	_, err := route.ExchangeClosedBootstrap(t.Context(), fixture, fixture.selection, pending.Request())
	if err == nil {
		t.Fatal("unreachable Entry unexpectedly issued tokens")
	}
	if got := route.ClosedBootstrapFailureDetail(err); got != "entry-carrier-tcp-dial-refused" {
		t.Fatalf("unreachable Entry boundary = %q; error = %v", got, err)
	}
}

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
