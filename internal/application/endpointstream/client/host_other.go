//go:build !linux

package client

import "errors"

// checkHost checks the selected local byte-stream host profile.
func checkHost() error {
	return errors.New("application stream requires the selected Linux host")
}
