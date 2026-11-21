package session

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/reachability"
	"github.com/dianabuilds/ardents-network/internal/route/introduction"
)

// Recipient is a copied exact Route choice, not authenticated current authority.
type Recipient struct {
	Node       [32]byte
	Generation uint64
	NotAfter   time.Time
}
type JoinIntent struct {
	Choice                  uint8
	Node                    [32]byte
	Generation              uint64
	Secret, Context         [32]byte
	Deadline, SetupDeadline time.Time
}
type CapsuleIntent struct {
	Slot, DeliveryNonce [32]byte
	Revision            uint64
	Expiry              time.Time
	RecipientKey        [32]byte
	Request             []byte
}

// RegistrationOwner lends only operations of the genuine original registration.
// The private constructors below never accept a caller-supplied implementation.
type RegistrationOwner interface {
	Close() error
	Withdraw(context.Context) error
	Done() <-chan struct{}
	Slot() [32]byte
	Receipt() (introduction.Receipt, error)
}
type Registration struct {
	owner RegistrationOwner
	facts introduction.RegistrationFacts
}

func (r Registration) Owner() RegistrationOwner              { return r.owner }
func (r Registration) Facts() introduction.RegistrationFacts { return r.facts }

// Prefix retains selection/Hosting roots until its physical Route owner joins.
// Zero values have no owner. Copies refer to the same original domain lifetime.
type Prefix struct {
	bootstrap         bool
	issue             func(context.Context, uint8, [][32]byte) error
	close             func() error
	replenish         func(context.Context) error
	done              <-chan struct{}
	register          func(context.Context, uint64) (Registration, error)
	recipient         func(uint8) (Recipient, error)
	join              func(context.Context, JoinIntent) (net.Conn, error)
	publishDescriptor func(context.Context, []byte) error
	lookupDescriptor  func(context.Context, [32]byte, *reachability.History) ([]byte, error)
	submitCapsule     func(context.Context, Recipient, []byte) error
	sealSubmitCapsule func(context.Context, Recipient, CapsuleIntent) error
}

func (p Prefix) Retained() bool  { return p.close != nil }
func (p Prefix) Bootstrap() bool { return p.bootstrap }
func (p Prefix) Close() error {
	if p.close == nil {
		return nil
	}
	return p.close()
}
func (p Prefix) Done() <-chan struct{} { return p.done }
func (p Prefix) Issue(ctx context.Context, class uint8, receivers [][32]byte) error {
	if p.issue == nil {
		return errors.New("route issuer exchange unavailable")
	}
	return p.issue(ctx, class, receivers)
}
func (p Prefix) Replenish(ctx context.Context) error {
	if p.replenish == nil {
		return errors.New("route replenishment absent")
	}
	return p.replenish(ctx)
}
func (p Prefix) Register(ctx context.Context, revision uint64) (Registration, error) {
	if p.register == nil || revision == 0 {
		return Registration{}, errors.New("route registration unavailable")
	}
	return p.register(ctx, revision)
}
func (p Prefix) Recipient(choice uint8) (Recipient, error) {
	if p.recipient == nil {
		return Recipient{}, errors.New("route Rendezvous selection unavailable")
	}
	return p.recipient(choice)
}
func (p Prefix) Join(ctx context.Context, intent JoinIntent) (net.Conn, error) {
	if p.join == nil {
		return nil, errors.New("route JOIN unavailable")
	}
	return p.join(ctx, intent)
}
func (p Prefix) PublishDescriptor(ctx context.Context, raw []byte) error {
	if p.publishDescriptor == nil {
		return errors.New("route Descriptor publication unavailable")
	}
	return p.publishDescriptor(ctx, raw)
}
func (p Prefix) LookupDescriptor(ctx context.Context, target [32]byte, history *reachability.History) ([]byte, error) {
	if p.lookupDescriptor == nil {
		return nil, errors.New("route Descriptor lookup unavailable")
	}
	return p.lookupDescriptor(ctx, target, history)
}
func (p Prefix) SubmitCapsule(ctx context.Context, recipient Recipient, raw []byte) error {
	if p.submitCapsule == nil {
		return errors.New("route capsule submission unavailable")
	}
	return p.submitCapsule(ctx, recipient, raw)
}
func (p Prefix) SealSubmitCapsule(ctx context.Context, recipient Recipient, intent CapsuleIntent) error {
	if p.sealSubmitCapsule == nil {
		return errors.New("route capsule input invalid")
	}
	return p.sealSubmitCapsule(ctx, recipient, intent)
}

// JoinContext owns the original retained choice across physical prefix openings.
type JoinContext struct {
	open  func(context.Context) (Prefix, error)
	close func() error
}

func (c JoinContext) Retained() bool { return c.close != nil }
func (c JoinContext) Close() error {
	if c.close == nil {
		return nil
	}
	return c.close()
}
func (c JoinContext) Open(ctx context.Context) (Prefix, error) {
	if c.open == nil {
		return Prefix{}, errors.New("route JOIN context absent")
	}
	return c.open(ctx)
}
