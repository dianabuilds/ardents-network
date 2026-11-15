package deployment

// NodeNetworkInputs is the portable public declaration of a Node's Network
// state location and trust inputs. It carries no authenticated State or duty.
type NodeNetworkInputs struct {
	Root                 string     `json:"root"`
	NetworkID            [32]byte   `json:"network_id"`
	Authorities          [][32]byte `json:"authorities"`
	Threshold            int        `json:"threshold"`
	ProfileAuthority     [32]byte   `json:"profile_authority"`
	ClockObservationFile string     `json:"clock_observation_file"`
}

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
