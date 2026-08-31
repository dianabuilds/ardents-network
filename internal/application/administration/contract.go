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
// Network, Route, credential, key, or transport facts from a caller.
type Interface interface {
	Publish(context.Context) error
	Withdraw(context.Context) error
}

// InterfaceFuncs adapts two cohesive operations to Interface.
type InterfaceFuncs struct {
	PublishFunc  func(context.Context) error
	WithdrawFunc func(context.Context) error
}

// Publish implements Interface.
func (owner InterfaceFuncs) Publish(ctx context.Context) error { return owner.PublishFunc(ctx) }

// Withdraw implements Interface.
func (owner InterfaceFuncs) Withdraw(ctx context.Context) error { return owner.WithdrawFunc(ctx) }
