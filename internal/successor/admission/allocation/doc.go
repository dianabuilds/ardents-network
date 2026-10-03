// Package allocation owns finite hourly permission-allocation decisions,
// exact request retry and the ARDALJ01 journal grammar. It holds no signing key,
// storage handle or live Network State authority. Custody serializes decisions,
// persists the encrypted successor and floor, and signs through its existing
// purpose-specific boundary before exposing a result. Holder request planning
// consumes the same role limits without acquiring allocation authority.
package allocation
