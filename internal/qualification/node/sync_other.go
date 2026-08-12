//go:build !linux

package node

func syncNodeDirectory(string) error { return nil }
