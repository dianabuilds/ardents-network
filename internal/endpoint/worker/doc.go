// Package worker owns the installed worker mechanism: the two root-installed
// artifact inventories, bounded read-only system-manager observation, unit
// property verification, activation through the installed sockets, exact
// process and cgroup identity, credentialed socket attachment, and the joined
// verified stop of one observed invocation.
//
// This package is mechanism, not authority. It grants nothing: launch
// permission, job reservation, the process-wide activation gate, readiness
// exchange, and Grant binding stay with the Endpoint composition, which
// supplies plain values and consumes the returned handles. Nothing here
// imports Endpoint, broker, or qualification; the API surface exists on Linux
// only, and this untagged doc.go keeps the package buildable on every
// platform.
package worker
