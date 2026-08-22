package stage6evidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/nameclaim"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

type controlRoleEvidence struct {
	Network          [32]byte
	Exchanges        []controlExchangeEvidence
	RelayRequests    uint32
	GatewayRequests  uint32
	GatewayAccepted  uint32
	ClaimEpoch       uint64
	ClaimMaximum     uint32
	ClaimThreshold   int
	ClaimAuthorities []byte
}

func runControlRoleCell(trace *traceRecord, secret [32]byte) error {
	network := [32]byte{9}
	now := time.Unix(1_800_000_000, 0).UTC()
	policy := namespace.Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour}
	corpus, err := newControlCorpus(network, now, policy)
	if err != nil {
		return err
	}
	gate, err := namespace.NewAdmission([32]byte{2}, network, 1, secret)
	if err != nil {
		return err
	}
	control, err := nameauthority.NewControl(network, gate, corpus.order, corpus.records,
		func() time.Time { return now }, policy)
	if err != nil {
		return err
	}
	authority := &evidenceControlAuthority{control: control}
	fixture, err := newResolutionFixture(authority)
	if err != nil {
		return err
	}
	defer fixture.close()
	isolations, results, err := executeControlOperations(fixture, gate, corpus.operations, now)
	if err != nil {
		return fmt.Errorf("private control authority transition was not admitted: %w", err)
	}
	observed, admissions, authorityResults, applyErr := authority.observation()
	if applyErr != nil || len(results) != len(corpus.operations) || len(observed) != len(corpus.operations) ||
		len(admissions) != len(corpus.operations) || len(authorityResults) != len(corpus.operations) {
		return errors.New("private control authority transition was not admitted")
	}
	envelopes := fixture.capture.evidence()
	if len(envelopes) != len(corpus.operations) {
		return errors.New("private control Relay evidence is incomplete")
	}
	exchanges := make([]controlExchangeEvidence, len(corpus.operations))
	outputs := make([]namespace.Record, len(corpus.operations))
	for index := range corpus.operations {
		if !cleanControlEnvelope(envelopes[index], isolations[index], observed[index]) {
			return errors.New("private control Relay observed a forbidden field")
		}
		outputs[index], err = namespace.DecodeRecord(results[index].State)
		observedResult := authorityResults[index]
		if err != nil || results[index].Class != observedResult.Class ||
			results[index].Generation != observedResult.Generation || results[index].Revision != observedResult.Revision ||
			!bytes.Equal(results[index].State, observedResult.State) {
			return errors.New("private control result is not canonical authority state")
		}
		exchanges[index] = controlExchangeEvidence{Isolation: isolations[index], Admission: admissions[index],
			Envelope: envelopes[index], Operation: observed[index], Result: results[index]}
	}
	relayRequests, _, _, _ := fixture.observations()
	requests, accepted, _ := fixture.control()
	evidence := controlRoleEvidence{Network: network, Exchanges: exchanges,
		RelayRequests: relayRequests, GatewayRequests: requests, GatewayAccepted: accepted,
		ClaimEpoch: corpus.order.MinimumEpoch, ClaimMaximum: corpus.order.MaximumClaims,
		ClaimThreshold: corpus.order.Threshold}
	for id, public := range corpus.order.Authorities {
		evidence.ClaimAuthorities = append(evidence.ClaimAuthorities, id[:]...)
		evidence.ClaimAuthorities = append(evidence.ClaimAuthorities, public...)
	}
	sortClaimAuthorities(evidence.ClaimAuthorities)
	raw, err := json.Marshal(evidence)
	if err != nil {
		return err
	}
	trace.Input, err = packRecords(corpus.records)
	if err != nil {
		return err
	}
	trace.Output, err = packRecords(outputs)
	trace.Auxiliary = raw
	trace.Fields = []string{"relay-location-only", "authority-control-only", "complete-control-lifecycle", "admitted"}
	return err
}

func runNamespaceForkCell(trace *traceRecord) error {
	first := evidenceClaim(0, [32]byte{1}, evidenceKey("namespace-fork-claim"))
	order, proof := evidenceClaimSet([]nameclaim.Claim{first})
	proof.Rule = "ardents-name-claim-order-v2"
	signEvidenceClaimClose(&proof)
	result, err := order.Verify(proof)
	if err != nil || result.Outcome != "fork" {
		return errors.New("incompatible Namespace rule was locally selected")
	}
	raw, err := json.Marshal(claimTraceEvidence{Primary: proof, Inputs: claimInputs(proof),
		Rejections: []claimRejectionEvidence{}})
	if err != nil {
		return err
	}
	trace.Auxiliary = raw
	trace.Values = []int64{int64(order.MinimumEpoch), int64(order.MaximumClaims), int64(order.Threshold)}
	for id, public := range order.Authorities {
		trace.Input = append(trace.Input, id[:]...)
		trace.Input = append(trace.Input, public...)
	}
	sortClaimAuthorities(trace.Input)
	trace.Fields = []string{"fork-unresolved", "different-rule", "no-local-selection"}
	return nil
}
