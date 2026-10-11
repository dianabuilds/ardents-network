//go:build linux

package receiver

import "testing"

// This table checks the purpose rule only; it supplies no authenticated peer,
// receiving admission or successful Carrier. Actual consumers check all three.
func TestIntroductionChildKeepsOriginalParentDomain(t *testing.T) {
	for domain := uint8(0); domain <= 5; domain++ {
		for purpose := uint8(0); purpose <= 8; purpose++ {
			want := domain == 1 && purpose == 5 || domain == 4 && purpose == 4
			if got := introductionChildPermitted(domain, purpose); got != want {
				t.Fatalf("parent domain %d purpose %d: permitted %v, want %v", domain, purpose, got, want)
			}
		}
	}
}
