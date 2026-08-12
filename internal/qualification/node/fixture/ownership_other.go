//go:build !linux

package fixture

func assignNodeOwnership(string) error { return nil }
