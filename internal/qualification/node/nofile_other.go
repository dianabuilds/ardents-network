//go:build !linux

package node

import "errors"

func lowerNodeNofile() error {
	return errors.New("node descriptor fault injection requires Linux")
}
