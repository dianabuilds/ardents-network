//go:build referencec2

package service_test

// referenceC2ID derives one deterministic opaque fixture identity from its
// marker. It does not identify a maintained product principal.
func referenceC2ID(value byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = value + byte(index)
	}
	return result
}
