// Package deployment owns bounded topology manifest validation, deterministic
// host-local plans, abstract three-Node observations, conservative clock-skew
// preflight, and exact Authority recovery coordination. It does not own host
// access, runtime mutation, Authority or repository truth, rollout journals,
// or reachability qualification.
package deployment
