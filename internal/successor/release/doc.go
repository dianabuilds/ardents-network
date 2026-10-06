// Package release verifies the bounded offline Release profile and owns its
// exclusive durable trust history. It serializes evaluations, retains verified
// Root advances on later refusal, and publishes immutable authorization only
// after target checks, durable metadata floors and the original caller check.
// Enrollment, distribution, installation, signing and execution are separate
// owners. Public result fields are observations, never authorization.
package release
