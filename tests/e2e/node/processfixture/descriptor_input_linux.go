package processfixture

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/tests/descriptorfixture"
)

// SignedDescriptor builds independent signed bytes for descriptor verification tests.
func SignedDescriptor(t testing.TB, facts admission.AuthorityFacts, caps uint32, introduction, profile [32]byte) ([]byte, [32]byte) {
	t.Helper()
	at := time.Now().UTC().Truncate(time.Second)
	end := at.Add(60 * time.Second)
	if facts.NotAfter.Before(end) {
		end = facts.NotAfter
	}
	return descriptorfixture.Signed(descriptorfixture.Input{
		AuthorityMarker: 0x71, InstanceMarker: 0x72,
		Network: facts.NetworkID, Profile: profile, Introduction: introduction,
		Slot: [32]byte{0x73}, RecipientKey: [32]byte{0x74},
		ValidFrom: at, PublicationEnd: end, RecipientEnd: end, Capabilities: caps,
	})
}
