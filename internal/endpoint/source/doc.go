//go:build linux

// Package source owns the live Source route opening of one duty context: the
// published read-only Handle, its in-progress replacement slot, and the exact
// acquisitions that bind one JOIN or resolution exchange to the handle current
// at admission. Lifecycle owns its operation reservation across prefix
// replacement. Its opening state is valid only under the owning dutyContext
// mutex; the retained Interior Set and role selection stay with the endpoint
// owner.
package source
