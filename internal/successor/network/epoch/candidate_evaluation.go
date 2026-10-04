package epoch

import (
	"errors"
	networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"
)

// evaluateInputs owns only decoding, signature examination and projection.
// Network owns input disposition; authenticated bytes stay with this adapter.
func evaluateInputs(config Policy, epoch epochEnvelope, inputs [][]byte) ([]nodeRecord, []rejection, networkdomain.CandidateView, error) {
	records := make([]nodeRecord, len(inputs))
	facts := make([]networkdomain.CandidateInput, len(inputs))
	for index, raw := range inputs {
		record, err := parseRecord(raw)
		if err != nil {
			continue
		}
		records[index] = record
		authentication := networkdomain.InvalidRecordSignature
		if record.signatureValid() {
			authentication = networkdomain.AuthenticatedRecord
		}
		facts[index] = networkdomain.CandidateInput{Authentication: authentication, Network: record.networkID, Node: record.nodeID,
			Key: record.keyID, ValidFrom: record.notBefore, ValidUntil: record.notAfter, Family: record.family,
			Endpoint: record.endpoint, Carrier: record.carrier, Capability: record.capability, Capacity: record.capacity}
	}
	policy := networkdomain.CandidatePolicy{Network: config.NetworkID, ValidFrom: epoch.validFrom, Profile: epoch.profile}
	for key := range config.Authorities {
		policy.AuthorityKeyIDs = append(policy.AuthorityKeyIDs, key)
	}
	view, err := networkdomain.EvaluateCandidates(policy, facts)
	if err != nil {
		return nil, nil, networkdomain.CandidateView{}, err
	}
	accepted := make([]nodeRecord, 0, len(inputs))
	for _, index := range view.AcceptedIndices() {
		accepted = append(accepted, records[index])
	}
	rejected := make([]rejection, 0, len(inputs))
	for _, item := range view.Rejections() {
		rejected = append(rejected, rejection{index: item.Index, code: item.Reason, raw: append([]byte(nil), inputs[item.Index]...)})
	}
	return accepted, rejected, view, nil
}

func candidateCarrierEligible(profile, carrier string) bool {
	return networkdomain.CandidateCarrierEligible(profile, carrier)
}

func candidateDomainSummaries(view networkdomain.CandidateView, epoch epochEnvelope) (map[string][2]uint32, error) {
	domains := make([]string, len(epoch.domains))
	for index, domain := range epoch.domains {
		domains[index] = domain.id
	}
	return view.DomainSummaries(epoch.networkID, epoch.number, epoch.assignmentSeed, domains)
}

func verifyCandidateSummaries(epoch epochEnvelope, view networkdomain.CandidateView) error {
	summary := view.Summary()
	if epoch.eligibleCount != summary.Count || epoch.eligibleCapacity != summary.Capacity || epoch.familyCount != summary.FamilyCount ||
		epoch.maxFamilyCount != summary.MaxFamilyCount || epoch.maxFamilyCapacity != summary.MaxFamilyCapacity {
		return errors.New("candidate view summaries are inconsistent")
	}
	computed, err := candidateDomainSummaries(view, epoch)
	if err != nil {
		return err
	}
	for _, expected := range epoch.domains {
		actual := computed[expected.id]
		if uint16(actual[0]) != expected.count || actual[1] != expected.capacity {
			return errors.New("role domain summaries are inconsistent")
		}
	}
	return nil
}
