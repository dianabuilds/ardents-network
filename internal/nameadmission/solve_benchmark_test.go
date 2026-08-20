package nameadmission_test

import "testing"

func BenchmarkRootClaimSolve(b *testing.B) {
	gate := admissionGate(b)
	challenges := make([]byte, b.N)
	for index := range challenges {
		challenges[index] = byte(index%255 + 1)
	}
	b.ResetTimer()
	for _, nonce := range challenges {
		_ = admissionProof(b, gate, "root-claim", nonce)
	}
}
