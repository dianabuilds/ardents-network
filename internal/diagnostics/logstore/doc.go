//go:build linux

// Package logstore owns the monitor's private retained log directory, writer
// lease, segment identity, rotation and joined close on Linux. Its detached
// policy and statistics describe retained bytes and delivery losses. The
// monitor owns its subprocess, console, status snapshot and panel.
package logstore
