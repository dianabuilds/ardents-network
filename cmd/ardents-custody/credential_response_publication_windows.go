//go:build windows

package main

// Windows does not expose directory sync. The staged regular file is flushed
// before its no-replace hard-link publication; native power-loss qualification
// remains a separate supported-platform gate.
func syncStableCustodyPublicDirectory(string) error { return nil }
