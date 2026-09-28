// Package state owns authenticated Network State admission, transition, and
// publication. Open verifies retained current, pending, and distribution state
// before starting any Source server or background work. Accept admits a
// current offline decision; Refresh resolves finite Source waves and may
// stage a pending one. Both obey the current/pending/conflict invariant and
// publish immutable decisions through the physical durable root.
//
// Epoch authenticates candidate bytes; Source owns finite TLS acquisition and
// distribution framing. State alone selects and commits the accepted decision.
// Closedprofile verifies the separate signed profile grammar, while State
// joins it to the current Epoch before exposing closed Route facts. Duty holds
// local Source exposure and serving-role conflicts; resource reports process
// pressure without gaining State authority. Readers receive copied views,
// and Wait and Close supervise and join the work owned by this State root.
package state
