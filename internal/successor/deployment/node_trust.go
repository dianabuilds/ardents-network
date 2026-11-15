package deployment

// ValidNodeTrustDeclaration checks only the shape of a Node's public Network
// inputs. Network State independently authenticates them at runtime.
func ValidNodeTrustDeclaration(network, profile [32]byte, authorities [][32]byte, threshold int) bool {
	if network == [32]byte{} || profile == [32]byte{} || len(authorities) == 0 || len(authorities) > 16 || threshold <= 0 || threshold > len(authorities) {
		return false
	}
	seen := make(map[[32]byte]bool, len(authorities))
	for _, key := range authorities {
		if key == [32]byte{} || seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}
