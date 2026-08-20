package stage6verify

import "bytes"

func verifyLifecycleTrace(trace traceRecord) bool {
	before, err := decodeRecords(trace.Input)
	if err != nil {
		return false
	}
	after, err := decodeRecords(trace.Output)
	if err != nil {
		return false
	}
	switch trace.Cell {
	case "A2":
		return len(before) == 0 && len(after) == 2 && baseRecord(after[0], 1, 1) &&
			after[0].Target == [32]byte{} && baseRecord(after[1], 1, 2) && after[1].Target == [32]byte{1}
	case "A3":
		return len(before) == 1 && len(after) == 3 && len(trace.Values) == 3 &&
			after[0].Lease == "active" && after[0].Revision == 2 &&
			after[1].Lease == "grace" && after[1].Revision == 2 &&
			after[2].Lease == "active" && after[2].Revision == 3 && after[2].LeaseExpires > after[1].LeaseExpires
	case "A4":
		return len(before) == 1 && len(after) == 3 && after[0].Lease == "grace" &&
			after[1].Lease == "released" && after[1].Generation == 1 && after[2].Generation == 2 &&
			after[2].Revision == 1 && after[2].Continuity == after[1].Continuity+1
	case "A5":
		return len(before) == 1 && len(after) == 2 && after[0].Parent == before[0].Name &&
			after[0].ParentGeneration == before[0].Generation && after[0].Authority != before[0].Authority &&
			after[0].LeaseExpires <= before[0].LeaseExpires && after[1].Lease == "released"
	case "B2":
		return len(before) == 1 && len(after) == 2 && after[0].RecoveryPolicy == [32]byte{} &&
			after[0].PendingPolicy == [32]byte{2} && after[0].PendingPolicyRev == 1 &&
			after[1].RecoveryPolicy == [32]byte{2} && after[1].RecoveryPolicyRev == 1 &&
			after[1].PendingPolicy == [32]byte{} && after[1].Revision == before[0].Revision+2
	case "B4":
		return len(before) == 1 && len(after) == 1 && recordsEqual(before[0], after[0]) &&
			len(trace.Fields) == 1 && trace.Fields[0] == "recovery-policy-absent"
	case "C0":
		return len(before) == 1 && len(after) == 1 && before[0].Target != [32]byte{} &&
			bytes.Equal(trace.Input, trace.Output)
	case "C1":
		return len(before) == 1 && len(after) == 1 && before[0].Generation == after[0].Generation &&
			after[0].Revision == before[0].Revision+1 && before[0].Authority == after[0].Authority &&
			before[0].Target != after[0].Target && after[0].Target == [32]byte{2}
	case "D5":
		return len(before) == 1 && len(after) == 1 && after[0].Generation == before[0].Generation &&
			after[0].Revision == before[0].Revision+1 && len(trace.Fields) == 1 && trace.Fields[0] == "stale-proof"
	default:
		return false
	}
}

func baseRecord(record decodedRecord, generation, revision uint64) bool {
	return record.Name == "alice" && record.Generation == generation && record.Revision == revision &&
		record.Lease == "active" && record.Consistency == "current" && record.Recovery == "stable"
}

func recordsEqual(left, right decodedRecord) bool {
	return bytes.Equal(encodeRecord(left), encodeRecord(right))
}
