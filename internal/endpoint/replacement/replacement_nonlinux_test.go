//go:build !linux

package replacement

import "errors"

func requireLinux() error { return errors.New("linux-only replacement behavior") }
