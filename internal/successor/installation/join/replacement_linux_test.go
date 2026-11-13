package join

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReplacementPreservesIdentityTrustAndLifetimeBounds(t *testing.T) {
	old := []byte(independentPlanVector())
	p, err := DecodePlan(old)
	if err != nil {
		t.Fatal(err)
	}
	encode := func(p Plan) []byte {
		raw, e := json.Marshal(p)
		if e != nil {
			t.Fatal(e)
		}
		return append(raw, '\n')
	}
	if checkReplacementPlan(old, old, false) == nil {
		t.Fatal("metadata-only successor accepted")
	}
	if err := checkReplacementPlan(old, old, true); err != nil {
		t.Fatal("program-only replacement refused", err)
	}
	p.Work.Tx++
	if err := checkReplacementPlan(old, encode(p), false); err != nil {
		t.Fatal("resource replacement refused", err)
	}
	for _, mutate := range []func(*Plan){
		func(p *Plan) { p.NodeID[0]++ }, func(p *Plan) { p.Network.NetworkID[0]++ },
		func(p *Plan) { p.Network.Authorities[0][0]++ }, func(p *Plan) { p.Network.ProfileAuthority[0]++ },
		func(p *Plan) { p.Network.Root += "-foreign" }, func(p *Plan) { p.ParticipationRoot += "-foreign" },
		func(p *Plan) { p.SpendRoot += "-foreign" }, func(p *Plan) { p.HostingRoot += "-foreign" },
		func(p *Plan) { p.Certificate += "-foreign" }, func(p *Plan) { p.PrivateKey += "-foreign" },
		func(p *Plan) { p.RetirementWaitMS-- }, func(p *Plan) { p.Network.ClockObservationFile += "-foreign" },
	} {
		next, e := DecodePlan(old)
		if e != nil {
			t.Fatal(e)
		}
		next.Work.Tx++
		mutate(&next)
		if checkReplacementPlan(old, encode(next), true) == nil {
			t.Fatal("program change admitted changed trust/identity/lifetime", string(encode(next)))
		}
	}
}

func TestReplacementSelectionRetainsExactInitialAndPredecessor(t *testing.T) {
	r, err := DecodeRequest([]byte(requestVector))
	if err != nil {
		t.Fatal(err)
	}
	sha := func(c string) string { return strings.Repeat(c, 64) }
	unit, err := renderUnit([]byte(Template), r, sha("1"))
	if err != nil {
		t.Fatal(err)
	}
	b := binding{Schema: "ardents-join-binding-v1", Invocation: "join", Generation: sha("1"), ProgramTarget: "ardents/linux-amd64/node", GenerationTarget: "ardents/linux-amd64/infrastructure/join", ProgramSHA256: sha("2"), TemplateSHA256: digest([]byte(Template)), InventorySHA256: sha("3"), TargetsSHA256: sha("4"), RequestSHA256: digest([]byte(requestVector)), PlanSHA256: sha("5"), UnitSHA256: digest(unit), CertificateSHA256: sha("6"), Account: "ardents-node", UID: 999, GID: 999, History: "/var/lib/release-history", Request: r}
	encode := func(v any) []byte {
		raw, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		return append(raw, '\n')
	}
	previous, err := decodeSelection(encode(b))
	if err != nil {
		t.Fatal(err)
	}
	next := b
	next.Schema = "ardents-join-replacement-configuration-v1"
	next.Generation = sha("7")
	rendered, err := renderUnit([]byte(Template), next.Request, next.Generation)
	if err != nil {
		t.Fatal(err)
	}
	next.UnitSHA256 = digest(rendered)
	outer := replacementBinding{Schema: "ardents-join-replacement-binding-v1", Configuration: next, InitialGeneration: b.Generation, PredecessorGeneration: b.Generation, PredecessorBindingSHA256: digest(previous.raw), RetirementSHA256: sha("8"), PredecessorAttempt: 1}
	selected, err := decodeSelection(encode(outer))
	if err != nil || selected.matchPredecessor(previous) != nil {
		t.Fatal("valid separate successor envelope", err)
	}
	if _, err := decodeBinding(encode(outer)); err == nil {
		t.Fatal("successor reinterpreted as original binding")
	}
	for _, mutate := range []func(*replacementBinding){
		func(r *replacementBinding) { r.PredecessorBindingSHA256 = sha("9") },
		func(r *replacementBinding) { r.InitialGeneration = sha("9") },
		func(r *replacementBinding) { r.Configuration.History += "-foreign" },
		func(r *replacementBinding) { r.Configuration.CertificateSHA256 = sha("9") },
		func(r *replacementBinding) { r.Configuration.UID--; r.Configuration.GID-- },
	} {
		changed := outer
		mutate(&changed)
		got, e := decodeSelection(encode(changed))
		if e == nil && got.matchPredecessor(previous) == nil {
			t.Fatal("foreign lineage retained", changed)
		}
	}
	if _, err := decodeSelection(append(encode(outer), byte(' '))); err == nil {
		t.Fatal("noncanonical successor accepted")
	}
}
