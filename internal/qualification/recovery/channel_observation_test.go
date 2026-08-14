package recovery

import (
	"bytes"
	"encoding/json"
	"testing"
)

func testChannelEvidence(t *testing.T, cell Cell, scope hostScopeEvidence) []byte {
	t.Helper()
	ref := func(local, remote, incarnation string) channelRefEvidence {
		value := channelRefEvidence{Adapter: scope.Adapter, Scope: scope.Commitment,
			NetworkScope: dockerChannelNetworkScope(scope, cell.FaultNetwork), Family: "tcp",
			Local: local, Remote: remote, Incarnation: incarnation}
		value.Commitment = channelRefCommitment(value)
		return value
	}
	projection := func(value any) json.RawMessage {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	initialRef := ref(cell.InitialCarrierLocal, cell.InitialCarrierRemote, cell.InitialCarrier)
	replacementRef := ref(cell.ReplacementCarrierLocal, cell.ReplacementCarrierRemote, cell.ReplacementCarrier)
	initialObservedAt := cell.HostStartedAtNanos + 1
	faultStartedAt, retiredAt := cell.HostStartedAtNanos+2, cell.HostStartedAtNanos+3
	faultCompletedAt, replacementObservedAt := cell.HostStartedAtNanos+4, cell.HostStartedAtNanos+5
	value := commonChannelEvidence{
		Initial: channelObservationEvidence{Ref: initialRef, State: "established",
			ObservedAtNanos: initialObservedAt, AdapterProjection: projection(dockerChannelProjection{
				Project: scope.AdapterProjection, Network: cell.FaultNetwork, HostProcess: cell.FaultContainer,
				Interface: cell.InitialCarrierInterface, SocketCommitment: cell.InitialCarrier,
				InterfaceIndex: cell.InitialCarrierInterfaceIndex, Inode: cell.InitialCarrierInode})},
		Replacement: channelObservationEvidence{Ref: replacementRef, State: "established",
			ObservedAtNanos: replacementObservedAt, AdapterProjection: projection(dockerChannelProjection{
				Project: scope.AdapterProjection, Network: cell.FaultNetwork, HostProcess: cell.FaultContainer,
				Interface: cell.ReplacementCarrierInterface, SocketCommitment: cell.ReplacementCarrier,
				InterfaceIndex: cell.ReplacementCarrierInterfaceIndex, Inode: cell.ReplacementCarrierInode})},
		Fault: channelFaultEvidence{Resource: initialRef, Operation: "retire-channel", Postcondition: "unavailable",
			InvocationStartedNanos: faultStartedAt, InvocationCompletedNanos: faultCompletedAt,
			ObservedAtNanos: faultCompletedAt, AdapterProjection: projection(dockerChannelFaultProjection{
				Project: scope.AdapterProjection, Controller: cell.FaultController, Network: cell.FaultNetwork,
				Interface: cell.InitialCarrierInterface, ControllerRemoved: cell.FaultControllerRemoved,
				Absent: cell.FaultResourceAbsent, CutAfterNanos: cell.CarrierCutAfterNanos,
				AbsenceAfterNanos: cell.AbsenceAfterNanos})},
		Retirement: channelStateEvidence{Resource: initialRef, State: "retired",
			ObservedAtNanos: retiredAt, AdapterProjection: projection(dockerChannelStateProjection{
				Project: scope.AdapterProjection, SocketCommitment: cell.RetiredCarrier,
				LeftEstablishedAfterNanos: 1})},
	}
	value.Initial.Commitment = channelObservationCommitment(value.Initial)
	value.Replacement.Commitment = channelObservationCommitment(value.Replacement)
	value.Fault.Commitment = channelFaultCommitment(value.Fault)
	value.Retirement.Commitment = channelStateCommitment(value.Retirement)
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestVerifyRejectsMutatedCommonChannelEvidence(t *testing.T) {
	for name, mutate := range map[string]func(*commonChannelEvidence){
		"fault targets replacement": func(value *commonChannelEvidence) {
			value.Fault.Resource = value.Replacement.Ref
			value.Fault.Commitment = channelFaultCommitment(value.Fault)
		},
		"replacement reuses channel": func(value *commonChannelEvidence) {
			value.Replacement.Ref = value.Initial.Ref
			value.Replacement.Commitment = channelObservationCommitment(value.Replacement)
		},
		"fault observation predates invocation": func(value *commonChannelEvidence) {
			value.Fault.ObservedAtNanos = value.Fault.InvocationStartedNanos - 1
			value.Fault.Commitment = channelFaultCommitment(value.Fault)
		},
		"Docker channel projection changed": func(value *commonChannelEvidence) {
			var projection dockerChannelProjection
			if err := json.Unmarshal(value.Initial.AdapterProjection, &projection); err != nil {
				t.Fatal(err)
			}
			projection.Project = "other"
			value.Initial.AdapterProjection, _ = json.Marshal(projection)
			value.Initial.Commitment = channelObservationCommitment(value.Initial)
		},
		"Docker channel projection has unknown field": func(value *commonChannelEvidence) {
			raw := bytes.TrimSuffix(value.Initial.AdapterProjection, []byte("}"))
			value.Initial.AdapterProjection = append(raw, []byte(`,"Unknown":true}`)...)
			value.Initial.Commitment = channelObservationCommitment(value.Initial)
		},
	} {
		t.Run(name, func(t *testing.T) {
			bundle := validEvidence(t)
			var value commonChannelEvidence
			if err := json.Unmarshal(bundle.Cells[0].ChannelEvidence, &value); err != nil {
				t.Fatal(err)
			}
			mutate(&value)
			var err error
			bundle.Cells[0].ChannelEvidence, err = json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if result := Verify(bundle); result.Verdict == "pass" {
				t.Fatalf("mutated common channel evidence passed: %+v", result)
			}
		})
	}
}

func TestVerifyRejectsMalformedCommonChannelEvidence(t *testing.T) {
	base := validEvidence(t).Cells[0].ChannelEvidence
	unknown := append([]byte(nil), bytes.TrimSuffix(base, []byte("}"))...)
	unknown = append(unknown, []byte(`,"Unknown":true}`)...)
	for name, raw := range map[string][]byte{
		"noncanonical":    append([]byte(" "), base...),
		"unknown field":   unknown,
		"multiple values": append(append([]byte(nil), base...), base...),
		"oversized":       bytes.Repeat([]byte("x"), (128<<10)+1),
	} {
		t.Run(name, func(t *testing.T) {
			bundle := validEvidence(t)
			bundle.Cells[0].ChannelEvidence = raw
			if result := Verify(bundle); result.Verdict == "pass" {
				t.Fatalf("malformed common channel evidence passed: %+v", result)
			}
		})
	}
}
