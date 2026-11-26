package node

import "github.com/dianabuilds/ardents-network/internal/pathcheck"

// IndependentRoots checks the declared State root together with the role roots.
func (network Network) IndependentRoots(roots ...string) bool {
	return pathcheck.DisjointRoots(append([]string{network.Root}, roots...)...)
}
