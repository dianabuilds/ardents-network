package administration

import "context"

// Operation is the closed local Service Administration request vocabulary.
type Operation string

const (
	Publish  Operation = "publish"
	Withdraw Operation = "withdraw"

	Published Outcome = "published"
	Withdrawn Outcome = "withdrawn"
)

// Outcome is the closed successful Administration result vocabulary.
type Outcome string

// Interface owns publication and withdrawal without accepting Service,
// Network, Route, credential, key, or transport facts from a caller. Each call
// is one non-retrying operation; nil means the requested transition committed,
// while any error is rendered as unavailable by the server Adapter. The
// context remains live for the operation and is cancelled when the local
// server closes. Implementations must serialize or reject conflicting
// transitions and make withdrawal idempotency explicit in their own outcome.
type Interface interface {
	Publish(context.Context) error
	Withdraw(context.Context) error
}
