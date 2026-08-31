//go:build referencec2

package main

// identifier derives one deterministic opaque fixture identity from its test
// marker. It exists only to keep roles distinct in the C-2 process scenario.
func identifier(value byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = value + byte(index)
	}
	return result
}
