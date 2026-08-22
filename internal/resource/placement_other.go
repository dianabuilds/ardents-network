//go:build !linux

package resource

func checkPlacement(profile) error { return errUnsupportedPlatform }
