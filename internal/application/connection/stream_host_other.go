//go:build !linux

package connection

import "errors"

// CheckByteStreamHost checks the selected local byte-stream host profile.
func CheckByteStreamHost() error {
	return errors.New("application stream requires the selected Linux host")
}
