package stage6verify

func evaluateHostileClaims(policy claimPolicy, proofs []claimEvidence) []string {
	if len(proofs) != 5 {
		return nil
	}
	out := make([]string, len(proofs))
	for index, proof := range proofs {
		candidate := policy
		if index == 1 {
			candidate.minimumEpoch++
		}
		out[index], _, _ = evaluateClaim(candidate, proof)
	}
	return out
}

func sameClaimClose(left, right claimEvidence) bool {
	return left.Rule == right.Rule && left.CutoffOffset == right.CutoffOffset &&
		left.InputRoot == right.InputRoot && left.InputLength == right.InputLength &&
		left.MaterializationRoot == right.MaterializationRoot &&
		left.MaterializationLength == right.MaterializationLength &&
		left.RejectionRoot == right.RejectionRoot && left.RejectionLength == right.RejectionLength
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalUint32(left, right []uint32) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
