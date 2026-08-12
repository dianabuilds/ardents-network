//go:build !linux

package node

import "errors"

func lowerNodeNofile() error {
	return errors.New("node descriptor limit injection requires Linux")
}
