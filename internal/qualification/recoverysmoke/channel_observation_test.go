package recoverysmoke

import (
	"encoding/json"
	"testing"
)

func TestFreezeCommonChannelEvidenceBindsObservedCarrier(t *testing.T) {
	scope, initial, replacement, fault := validChannelProducerInput()
	raw, err := freezeCommonChannelEvidence(scope, "campaign_carrier_net", "rendezvous", "controller",
		initial, replacement, fault, 2, 11)
	if err != nil {
		t.Fatal(err)
	}
	var value commonChannelEvidence
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if value.Initial.Ref.Incarnation != initial.SocketIDSHA256 ||
		value.Replacement.Ref.Incarnation != replacement.SocketIDSHA256 ||
		value.Fault.Resource != value.Initial.Ref || value.Retirement.Resource != value.Initial.Ref ||
		value.Initial.Commitment == [32]byte{} || value.Fault.Commitment == [32]byte{} {
		t.Fatalf("common channel evidence is incomplete: %+v", value)
	}
}

func TestFreezeCommonChannelEvidenceRejectsIncompleteInput(t *testing.T) {
	for name, mutate := range map[string]func(*carrierObservation, *carrierFaultOutcome){
		"missing channel": func(initial *carrierObservation, _ *carrierFaultOutcome) {
			initial.SocketIDSHA256 = ""
		},
		"missing retirement": func(_ *carrierObservation, fault *carrierFaultOutcome) {
			fault.socketRetired = false
		},
	} {
		t.Run(name, func(t *testing.T) {
			scope, initial, replacement, fault := validChannelProducerInput()
			mutate(&initial, &fault)
			if _, err := freezeCommonChannelEvidence(scope, "campaign_carrier_net", "rendezvous", "controller",
				initial, replacement, fault, 2, 11); err == nil {
				t.Fatal("incomplete common channel evidence was frozen")
			}
		})
	}
}

func validChannelProducerInput() (hostScopeEvidence, carrierObservation, carrierObservation, carrierFaultOutcome) {
	scope := hostScopeEvidence{Adapter: "docker-compose-v1", AdapterProjection: "campaign", Commitment: [32]byte{1}}
	initial := carrierObservation{SocketIDSHA256: "initial", LocalAddress: "172.31.21.13:50001",
		RemoteAddress: carrierRemote, Inode: 1, InterfaceName: "eth0", InterfaceIndex: 3}
	replacement := carrierObservation{SocketIDSHA256: "replacement", LocalAddress: "172.31.21.13:50002",
		RemoteAddress: carrierRemote, Inode: 2, InterfaceName: "eth2", InterfaceIndex: 4}
	fault := carrierFaultOutcome{commitment: "initial", retiredCommitment: "initial",
		faultAt: 3, completedAt: 10, cutAfter: 1, absenceAfter: 2, socketRetiredAt: 8,
		retiredAfter: 2, hostFaultAt: 3, hostCompletedAt: 10, hostRetiredAt: 8,
		controllerRemoved: true, resourceAbsent: true, socketRetired: true}
	return scope, initial, replacement, fault
}
