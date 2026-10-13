//go:build linux

package stock

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
)

func TestPreparationMissingPreservesOrderMultiplicityAndIrreversibleRights(t *testing.T) {
	o, h, presentation := issuedStockFixture(t)
	p, err := o.Prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	a := token.ClosedTokenContext{NetworkID: h.profile.NetworkID, ProfileDigest: h.profile.Digest,
		IssuerNodeID: h.profile.IssuerNodeID, ReceiverNodeID: presentation.RecipientNodeID,
		ReceiverDutyGeneration: presentation.RecipientDutyGeneration, Class: 2, WindowStart: h.now.Truncate(time.Hour)}
	b := a
	b.ReceiverNodeID = fixtureID(7)
	before := o.Status()
	missing, err := p.Missing([]token.ClosedTokenContext{a, a, b})
	if err != nil || len(missing) != 2 || missing[0] != a || missing[1] != b {
		t.Fatal("ordered deficit differs", missing, err)
	}
	if o.Status() != before {
		t.Fatal("stock observation reserved or refunded rights")
	}
	if _, err := o.Begin(nextIntent(h)); err == nil {
		t.Fatal("unrelated issuance entered original preparation")
	}
	wrong := a
	wrong.ProfileDigest = fixtureID(99)
	if _, err := p.Missing([]token.ClosedTokenContext{wrong}); err == nil {
		t.Fatal("foreign profile supplied stock")
	}
	raw, err := o.Take(t.Context(), presentation, 2)
	clear(raw)
	if err != nil {
		t.Fatal(err)
	}
	missing, err = p.Missing([]token.ClosedTokenContext{a, a, b})
	if err != nil || len(missing) != 3 || missing[0] != a || missing[1] != a || missing[2] != b {
		t.Fatal("spent token reappeared in planning", missing, err)
	}
	if o.Status().Remaining != before.Remaining || o.Status().BootstrapRemaining != before.BootstrapRemaining {
		t.Fatal("presentation or deficit query renewed issuance allocation")
	}
}
