//go:build linux

package endpoint

import (
	"testing"
	"time"

	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// Explicit Close supplies a fully joined expired Source without a 120-second
// unit-test sleep. Issuance, resolution and Introduction preparation stay real;
// this does not qualify installed workers or the idle timer itself.
func TestTextIntroductionReopensJoinedSource(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			reader, _, destination := joinedNetworkFixture(t, carrier)
			job := liveCapsuleJob(t, reader)
			until := time.Now().Add(time.Minute).Unix()
			bounds := [3]int64{until, until, until}
			first, err := reader.prepareIntroduction(t.Context(), job, destination, bounds)
			if err != nil {
				t.Fatal(err)
			}
			clear(first.operation)
			reader.mu.Lock()
			prefix, permission := reader.source.currentLocked(), reader.tokens.Permission
			reader.mu.Unlock()
			if err := prefix.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-prefix.Done():
			default:
				t.Fatal("Source Close did not join")
			}
			second, err := reader.prepareIntroduction(t.Context(), job, destination, bounds)
			if err != nil {
				t.Fatalf("explicit read after joined Source: %v", err)
			}
			clear(second.operation)
			reader.mu.Lock()
			valid := reader.source.currentLocked() != nil && reader.source.currentLocked() != prefix && reader.tokens.Permission == permission && permission.Batches == 2 &&
				permission.Pending == nil && reader.tokens.Issuance == nil && reader.resolution == nil && reader.source.opening == nil
			reserved := permission.Reserved
			maxima := permission.Accepted.Maxima
			reader.mu.Unlock()
			if !valid {
				t.Fatal("reopen replaced permission, repeated bootstrap or retained a flight")
			}
			for class, count := range reserved {
				if count > maxima[class] {
					t.Fatal("reopen exceeded allocation")
				}
			}
		})
	}
}
