package deployment

import "testing"

func TestNodeTrustDeclarationRequiresIndependentNonzeroAuthorities(t *testing.T) {
	network, profile, first, second := [32]byte{1}, [32]byte{2}, [32]byte{3}, [32]byte{4}
	if !ValidNodeTrustDeclaration(network, profile, [][32]byte{first, second}, 2) {
		t.Fatal("valid declaration refused")
	}
	for _, tc := range []struct {
		network, profile [32]byte
		authorities      [][32]byte
		threshold        int
	}{
		{[32]byte{}, profile, [][32]byte{first}, 1},
		{network, [32]byte{}, [][32]byte{first}, 1},
		{network, profile, nil, 1},
		{network, profile, [][32]byte{first, first}, 1},
		{network, profile, [][32]byte{first, {}}, 1},
		{network, profile, [][32]byte{first}, 0},
		{network, profile, [][32]byte{first}, 2},
		{network, profile, make([][32]byte, 17), 1},
	} {
		if ValidNodeTrustDeclaration(tc.network, tc.profile, tc.authorities, tc.threshold) {
			t.Fatal("invalid trust declaration accepted")
		}
	}
}
