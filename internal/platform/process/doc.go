// Package process retains original Linux process observations for Endpoint
// invocation and Source repair. Invocation observes the original proc
// directory, start clock, executable, arguments, credentials, cgroup and
// InvocationID. Retain observes the fixed predecessor as root. RetainSelf
// observes the actual non-root caller.
//
// Source recovery separately checks that no other process retains the original
// Source principal. State still owns retained leases and its original repair
// decision. Observations grant no signal, Release or startup authority.
package process

import "errors"

var errBinding = errors.New("installation process: observation mismatch")
