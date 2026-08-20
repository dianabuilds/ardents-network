package stage6verify

import "crypto/sha256"

type connectionCellEvidence struct {
	Initial        connectionBinding
	Replacement    connectionBinding
	NameOrigin     bool
	ClientClass    string
	PublisherClass string
	Target         [32]byte
}

type connectionBinding struct {
	Name             string
	Generation       uint64
	Revision         uint64
	Authority        string
	Target           [32]byte
	ParentName       string
	ParentGeneration uint64
	RecordDigest     [32]byte
	Commitment       [32]byte
}

func verifyConnectionTrace(trace traceRecord) bool {
	var evidence connectionCellEvidence
	initial, err := decodeRecords(trace.Input)
	if err != nil || len(initial) != 1 {
		return false
	}
	replacement, err := decodeRecords(trace.Output)
	if err != nil || len(replacement) != 1 || decodeNestedJSON(trace.Auxiliary, &evidence) != nil {
		return false
	}
	if !bindingMatchesRecord(evidence.Initial, initial[0]) ||
		!bindingMatchesRecord(evidence.Replacement, replacement[0]) ||
		replacement[0].Generation != initial[0].Generation || replacement[0].Revision != initial[0].Revision+1 ||
		replacement[0].Target == initial[0].Target || evidence.Target != initial[0].Target ||
		!equalStrings(trace.Fields, []string{evidence.ClientClass, evidence.PublisherClass}) {
		return false
	}
	if trace.Cell == "C2" {
		return evidence.NameOrigin && evidence.ClientClass == "abrupt connection loss"
	}
	return !evidence.NameOrigin && evidence.ClientClass == "clean service connection close" &&
		evidence.PublisherClass == "clean service connection close"
}

func bindingMatchesRecord(binding connectionBinding, record decodedRecord) bool {
	digest := sha256.Sum256(encodeRecord(record))
	commitment := sha256.Sum256(append([]byte("ardents-h3-name-destination-binding-v1\x00"), digest[:]...))
	return binding.Name == record.Name && binding.Generation == record.Generation &&
		binding.Revision == record.Revision && binding.Authority == record.Authority &&
		binding.Target == record.Target && binding.ParentName == record.Parent &&
		binding.ParentGeneration == record.ParentGeneration && binding.RecordDigest == digest &&
		binding.Commitment == commitment
}
