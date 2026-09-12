//go:build linux

package endpoint

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// Explicit Close supplies a fully joined expired Source without a 120-second
// unit-test sleep. Issuance, resolution and Introduction preparation stay real;
// this does not qualify installed workers or the idle timer itself.
func TestTextIntroductionReopensJoinedSource(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			reader, _, destination := textJoinedNetworkFixture(t, carrier)
			job := liveTextCapsuleJob(t, reader)
			until := time.Now().Add(time.Minute).Unix()
			bounds := [3]int64{until, until, until}
			first, err := reader.prepareTextIntroduction(t.Context(), job, destination, bounds)
			if err != nil {
				t.Fatal(err)
			}
			clear(first.operation)
			reader.mu.Lock()
			prefix, permission := reader.prefix, reader.permission
			reader.mu.Unlock()
			if err := prefix.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-prefix.Done():
			default:
				t.Fatal("Source Close did not join")
			}
			second, err := reader.prepareTextIntroduction(t.Context(), job, destination, bounds)
			if err != nil {
				t.Fatalf("explicit read after joined Source: %v", err)
			}
			clear(second.operation)
			reader.mu.Lock()
			valid := reader.prefix != nil && reader.prefix != prefix && reader.permission == permission && permission.batches == 2 &&
				permission.pending == nil && reader.issuance == nil && reader.resolution == nil && reader.prefixOpening == nil
			reserved := permission.reserved
			maxima := permission.accepted.Maxima
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
