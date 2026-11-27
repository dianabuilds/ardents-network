//go:build linux

// Package permission owns the original installed Endpoint permission files.
// It publishes an exact request without replacement, reads the original
// response through custody checks, and retires an admitted pair. Endpoint
// supplies the live Startup check; Admission Stock decides response validity.
package permission
