package forwarding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

// Detached closed byte vectors only; no signed Release, original retirement,
// native installation or startup authority is supplied by these codec controls.
func replacementSelectionVector(t *testing.T) (generationSelection, generationSelection) {
	t.Helper()
	b, raw, _ := selectedGenerationVector(t)
	old, err := decodeSelection(raw)
	if err != nil {
		t.Fatal(err)
	}
	b.Schema, b.Generation = "ardents-forwarding-replacement-configuration-v1", digest([]byte("new generation"))
	unit, err := RenderUnit([]byte(Template), b.Request, b.Generation)
	if err != nil {
		t.Fatal(err)
	}
	b.UnitSHA256 = digest(unit)
	configuration, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	vector := fmt.Appendf(nil, "{\"schema\":\"ardents-forwarding-replacement-binding-v1\",\"configuration\":%s,\"initial_generation\":\"%s\",\"predecessor_generation\":\"%s\",\"predecessor_binding_sha256\":\"%s\",\"retirement_sha256\":\"%s\",\"predecessor_attempt\":0}\n", configuration, old.binding.Generation, old.binding.Generation, digest(old.raw), digest([]byte("retirement")))
	next, err := decodeSelection(vector)
	if err != nil {
		t.Fatal("independent replacement selection vector", err)
	}
	return old, next
}

func TestForwardingSelectionKeepsInitialAndReplacementGrammarsDistinct(t *testing.T) {
	old, next := replacementSelectionVector(t)
	if next.matchPredecessor(old) != nil || next.initialGeneration() != old.binding.Generation || next.binding.Generation == old.binding.Generation {
		t.Fatal("exact predecessor lost")
	}
	if _, err := decodeBinding(next.raw); err == nil {
		t.Fatal("replacement became initial provenance")
	}
	configuration, _ := json.Marshal(next.binding)
	if _, err := decodeBinding(append(configuration, '\n')); err == nil {
		t.Fatal("replacement configuration became initial binding")
	}
	for _, raw := range [][]byte{
		bytes.TrimSuffix(next.raw, []byte{'\n'}), append(bytes.Clone(next.raw), '\n'),
		bytes.Replace(next.raw, []byte("{\"schema\""), []byte("{ \"schema\""), 1),
		bytes.Replace(next.raw, []byte("\"predecessor_attempt\":0"), []byte("\"predecessor_attempt\":4294967295"), 1),
		bytes.Replace(next.raw, []byte("\"predecessor_attempt\":0"), []byte("\"predecessor_attempt\":63"), 1),
		bytes.Replace(next.raw, []byte("\"predecessor_attempt\":0"), []byte("\"predecessor_attempt\":0,\"extra\":0"), 1),
		bytes.Replace(next.raw, []byte("\"predecessor_attempt\":0"), []byte("\"predecessor_attempt\":1,\"predecessor_attempt\":0"), 1),
	} {
		if _, err := decodeSelection(raw); err == nil {
			t.Fatal("noncanonical or out-of-bound selection admitted")
		}
	}
	input := bytes.Clone(next.raw)
	copy, err := decodeSelection(input)
	if err != nil {
		t.Fatal(err)
	}
	before := digest(copy.raw)
	input[0] ^= 1
	if digest(copy.raw) != before {
		t.Fatal("caller mutation rebound retained selection bytes")
	}
}

func TestForwardingSelectionRequiresExactRetainedPredecessor(t *testing.T) {
	old, next := replacementSelectionVector(t)
	for _, fault := range []string{"generation", "binding", "initial", "State", "participation", "spend", "Hosting", "credential", "clock", "account", "principal", "Release-history", "certificate", "template"} {
		t.Run(fault, func(t *testing.T) {
			bad := next
			r := *next.replacement
			bad.replacement = &r
			switch fault {
			case "generation":
				r.PredecessorGeneration = digest([]byte("foreign"))
			case "binding":
				r.PredecessorBindingSHA256 = digest([]byte("foreign"))
			case "initial":
				r.InitialGeneration = digest([]byte("foreign"))
			case "State":
				bad.binding.Request.StateRoot += "-other"
			case "participation":
				bad.binding.Request.ParticipationRoot += "-other"
			case "spend":
				bad.binding.Request.SpendRoot += "-other"
			case "Hosting":
				bad.binding.Request.HostingRoot += "-other"
			case "credential":
				bad.binding.Request.CredentialDirectory += "-other"
			case "clock":
				bad.binding.Request.Clock += "-other"
			case "account":
				bad.binding.Account = "other"
			case "principal":
				bad.binding.UID++
			case "Release-history":
				bad.binding.History += "-other"
			case "certificate":
				bad.binding.CertificateSHA256 = digest([]byte("foreign"))
			case "template":
				bad.binding.TemplateSHA256 = digest([]byte("foreign"))
			}
			if bad.matchPredecessor(old) == nil {
				t.Fatal("changed retained owner accepted", fault)
			}
		})
	}
	next.binding.Request.Plan += "-new-public-input"
	next.binding.Request.Certificate += "-new-public-input"
	next.binding.Request.PrivateKey += "-new-protected-input"
	if next.matchPredecessor(old) != nil {
		t.Fatal("fresh delivery paths changed retained runtime roots")
	}
}

func TestForwardingResourceOnlyPlanNeedsActualWorkOrTerminationDelta(t *testing.T) {
	_, _, files := selectedGenerationVector(t)
	old := files["forwarding-plan.json"]
	p, err := DecodePlan(old)
	if err != nil {
		t.Fatal(err)
	}
	if checkForwardPlan(old, old, false) == nil {
		t.Fatal("version-only change became resource update")
	}
	if err := checkForwardPlan(old, old, true); err != nil {
		t.Fatal("actual program update refused", err)
	}
	for _, resource := range []string{"work", "termination"} {
		next := p
		if resource == "work" {
			next.Work.Tx++
		} else {
			next.Termination.Rx++
		}
		raw, _ := json.Marshal(next)
		if err := checkForwardPlan(old, append(raw, '\n'), false); err != nil {
			t.Fatal("actual resource delta refused", resource, err)
		}
	}
	for _, fault := range []string{"Node", "pin", "root", "clock", "credential", "protection", "lifetime"} {
		next := p
		next.Network.Authorities = append([][32]byte(nil), p.Network.Authorities...)
		next.Work.Tx++
		switch fault {
		case "Node":
			next.NodeID[0] ^= 1
		case "pin":
			next.Network.Authorities[0][0] ^= 1
		case "root":
			next.HostingRoot += "-other"
		case "clock":
			next.Network.ClockObservationFile += "-other"
		case "credential":
			next.PrivateKey += "-other"
		case "protection":
			next.ProcessProfile += "-other"
		case "lifetime":
			next.RetirementWaitMS++
		}
		raw, _ := json.Marshal(next)
		if checkForwardPlan(old, append(raw, '\n'), false) == nil {
			t.Fatal("resource update changed another owner", fault)
		}
	}
}
