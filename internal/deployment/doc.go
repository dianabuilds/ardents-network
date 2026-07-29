// Package deployment owns bounded topology manifest validation, deterministic
// host-local plans, abstract three-Node observations, conservative clock-skew
// preflight, exact Authority recovery coordination, and crash-resumable
// Deployment Fence Transactions over consumer-owned adapters. It does not own
// host access, concrete runtime/network mutation, Authority or repository
// truth, rollout journals, or reachability qualification.
package deployment
