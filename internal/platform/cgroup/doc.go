// Package cgroup observes actual Linux cgroup v2 scopes and their descendants.
// kernel_linux.go observes the bounded scope inventory for Endpoint replacement.
// quiescence_linux.go checks the one fixed unit of a retained Source or Node
// repair. Source repair also requires the separate process owner's principal
// check. Sibling Node roles retain their independent scopes.
//
// Callers verify the admitted unit, exact process and semantic roots. Kernel
// emptiness alone grants no process management, software authorization or
// joined completion for an earlier invocation.
package cgroup

import "errors"

var errBinding = errors.New("installation cgroup observation differs")
