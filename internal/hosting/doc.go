// Package hosting owns one durable provider-period allowance and the shared
// reservations against it. It charges whole-interface counters, preserves used
// and reserved floors across reopen, and refuses ambiguous continuity. A
// reservation owns work and termination capacity until its consumer joins and
// releases it; copying a handle cannot create another refund. Hosting grants no
// Admission token, State authority, process placement or duty assignment.
package hosting
