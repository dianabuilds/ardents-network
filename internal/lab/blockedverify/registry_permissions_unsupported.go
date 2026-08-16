//go:build !linux && !windows

package blockedverify

import "errors"

func protectRegistryTree(string) error {
	return errors.New("replay registry ownership checks are unsupported on this platform")
}
