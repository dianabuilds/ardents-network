//go:build !linux

package fixture

import "errors"

func assignNodeOwnership(string) error {
	return errors.New("node fixture Linux UID ownership requires a Linux host")
}
