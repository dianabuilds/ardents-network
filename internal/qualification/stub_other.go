//go:build !linux

package qualification

// The qualification package owns types shared between Endpoint and the
// installed qualification command. All production implementations require
// the Linux installed-worker environment. This stub keeps the package
// visible to the architecture gate on non-Linux platforms.