package runtime

import (
	"context"
	"errors"
	"sync"
	"time"

	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/instance"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
)

// Config transfers the original two physical prefixes to this Publisher. Their
// shared installation selection and Hosting root remain composition-owned.
type Config struct {
	Operation            *executionruntime.Operation
	Binding              *instance.Binding
	Source, Introduction *prefix.Prefix
	// Responder transfers one prepared role-3 selection and reservation on
	// successful New. Physical opening remains bound to the original Source.
	Responder  prefix.Config
	Duty       network.RetainedDuty
	Deadline   time.Time
	Exclusions []route.Member
}

type Publisher struct {
	mu                            sync.Mutex
	config                        Config
	lifetime                      *instance.PublicationLifetime
	credential                    publication.Credential
	administration                executionruntime.AdministrationContext
	history                       *recipientHistory
	ctx                           context.Context
	cancel                        context.CancelFunc
	physicalContext               context.Context
	cancelPhysical                context.CancelFunc
	stopLifetime                  func() bool
	stopOperation                 func() bool
	operationCallback             chan struct{}
	lifetimeCallback              chan struct{}
	current, pending, predecessor *registrationPair
	revision                      uint64
	flight                        bool
	flights                       sync.WaitGroup
	closed                        bool
	closeOnce                     sync.Once
	closeErr                      error
	failure                       error
	watcherDone, schedulerDone    chan struct{}
	wake                          chan struct{}
	responderGate                 chan struct{}
	responder                     *prefix.Prefix
	responderAttempted            bool
	responderFailure              error
	responderCancel               context.CancelFunc
	sourceBorrow, responderBorrow *prefix.Borrow
	sourceUsersDone               chan struct{}
	sourceUsersErr                error
	initialDeliveries             map[[32]byte]*initialDelivery
	initialUsers                  sync.WaitGroup
}

func New(ctx context.Context, config Config) (*Publisher, error) {
	if ctx == nil || config.Operation == nil || config.Binding == nil || config.Source == nil || config.Introduction == nil || config.Source == config.Introduction || config.Responder.Release == nil || config.Responder.Current == nil || config.Responder.Present == nil || config.Responder.Leg.EntryMember.RoleDomain != 3 || config.Deadline != config.Deadline.UTC().Truncate(time.Second) || !time.Now().Before(config.Deadline) {
		return nil, errors.New("live Publisher inputs absent")
	}
	if err := errors.Join(ctx.Err(), config.Operation.CheckPublisher()); err != nil {
		return nil, err
	}
	administration, err := config.Operation.AdministrationContext()
	if err != nil {
		return nil, err
	}
	history, err := retainRecipientHistory(administration, config.Operation)
	if err != nil {
		return nil, err
	}
	lifetime, err := config.Binding.BeginPublication(config.Operation.Context())
	if err != nil {
		return nil, err
	}
	credential, err := lifetime.Credential()
	if err != nil {
		lifetime.Close()
		return nil, err
	}
	if bound := credential.Delegation().NotAfter; bound.Before(config.Deadline) {
		config.Deadline = bound
	}
	// Local stop seals Publication and interrupts its users. Physical parents
	// retain the original caller and deadline through their bounded retirement;
	// original caller/Operation/Instance loss still interrupts both lifetimes.
	physicalContext, cancelPhysical := context.WithDeadline(ctx, config.Deadline)
	child, cancel := context.WithCancel(physicalContext)
	p := &Publisher{config: config, lifetime: lifetime, credential: credential, administration: administration, history: history, ctx: child, cancel: cancel, physicalContext: physicalContext, cancelPhysical: cancelPhysical, lifetimeCallback: make(chan struct{}), operationCallback: make(chan struct{}), watcherDone: make(chan struct{}), schedulerDone: make(chan struct{}), wake: make(chan struct{}, 1), sourceUsersDone: make(chan struct{})}
	p.config.Exclusions = append([]route.Member(nil), config.Exclusions...)
	p.responderGate = make(chan struct{}, 1)
	p.responderGate <- struct{}{}
	p.sourceBorrow, err = config.Source.BorrowLifetime(child, p.stop, p.joinSourceUsers)
	if err != nil {
		cancel()
		cancelPhysical()
		lifetime.Close()
		return nil, err
	}
	err = config.Source.CommitPair(nil, func(checkOriginal func() error) error {
		return errors.Join(ctx.Err(), child.Err(), lifetime.Context().Err(), checkOriginal(), config.Operation.CheckPublisher())
	})
	if err != nil {
		cancel()
		cancelPhysical()
		lifetime.Close()
		// No producer was started. Release the retained physical claim only
		// after its empty original-user barrier is terminal, outside Prefix locks.
		close(p.sourceUsersDone)
		p.sourceBorrow.ReturnJoined()
		return nil, err
	}
	p.initialDeliveries = make(map[[32]byte]*initialDelivery)
	p.stopLifetime = context.AfterFunc(lifetime.Context(), func() { defer close(p.lifetimeCallback); cancelPhysical() })
	p.stopOperation = context.AfterFunc(config.Operation.Context(), func() { defer close(p.operationCallback); cancelPhysical() })
	go func() {
		defer close(p.watcherDone)
		select {
		case <-child.Done():
		case <-config.Source.Done():
		}
		p.stop()
		p.drain()
	}()
	go p.schedule()
	return p, nil
}

func (p *Publisher) check(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Publisher caller absent")
	}
	_, err := p.lifetime.Credential()
	return errors.Join(ctx.Err(), p.ctx.Err(), p.administration.Check(p.config.Operation), err)
}

// Link is visible only under the current exact acknowledged accepting pair.
func (p *Publisher) Link(ctx context.Context) (string, error) {
	if p == nil {
		return "", errors.New("Publisher absent")
	}
	p.mu.Lock()
	pair, closed := p.current, p.closed
	p.mu.Unlock()
	if closed || pair == nil {
		return "", errors.New("Publisher has no accepting pair")
	}
	if err := pair.check(p, ctx); err != nil {
		return "", err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.current != pair || pair.firstACK.IsZero() {
		return "", errors.New("Publisher accepting pair changed")
	}
	if err := p.check(ctx); err != nil {
		return "", err
	}
	value := p.credential.Delegation()
	return publication.TargetLink(value.Network, value.Target)
}

// Publish retries one exact pending flight or returns the original current
// Link; it does not create a replacement for an acknowledged pair.
func (p *Publisher) Publish(ctx context.Context) (string, error) { return p.publish(ctx, false) }

// Refresh reserves a new real slot/key and higher revision. The previous pair
// remains current until the exact replacement Store ACK passes final checks.
func (p *Publisher) Refresh(ctx context.Context) (string, error) { return p.publish(ctx, true) }

func (p *Publisher) stop() {
	p.mu.Lock()
	p.closed = true
	// Seal original registration outcomes before interrupting our owned
	// contexts. Stop does not join; drain retains every physical owner.
	for _, pair := range []*registrationPair{p.pending, p.current, p.predecessor} {
		if pair != nil && pair.registration != nil {
			pair.registration.Stop()
		}
	}
	p.cancel()
	p.mu.Unlock()
}

func (p *Publisher) Close() error {
	if p == nil {
		return nil
	}
	p.stop()
	p.drain()
	<-p.watcherDone
	return p.closeErr
}

func (p *Publisher) drain() {
	p.closeOnce.Do(func() {
		<-p.schedulerDone
		p.flights.Wait()
		p.initialUsers.Wait()
		p.mu.Lock()
		p.closeErr = p.failure
		p.mu.Unlock()
		// No producer remains. Physical registrations/prefixes join before any
		// private recipient is erased or the Instance reservation is returned.
		for _, pair := range []*registrationPair{p.pending, p.current, p.predecessor} {
			if pair != nil {
				p.closeErr = errors.Join(p.closeErr, pair.close())
			}
		}
		// The Prefix callback waits only for original children/private users,
		// never for its own recursive Close. Both physical claims are returned
		// after this barrier, before closing the prefixes themselves.
		p.sourceUsersErr = p.closeErr
		close(p.sourceUsersDone)
		p.sourceBorrow.ReturnJoined()
		p.responderBorrow.ReturnJoined()
		p.closeErr = errors.Join(p.closeErr, p.responder.Close())
		if p.responderCancel != nil {
			p.responderCancel()
		}
		if p.config.Responder.Release != nil {
			p.closeErr = errors.Join(p.closeErr, p.config.Responder.Release())
		}
		p.closeErr = errors.Join(p.closeErr, p.config.Introduction.Close(), p.config.Source.Close())
		p.cancelPhysical()
		p.lifetime.Close()
		p.closeErr = errors.Join(p.closeErr, p.config.Binding.Close())
		if !p.stopLifetime() {
			<-p.lifetimeCallback
		}
		if !p.stopOperation() {
			<-p.operationCallback
		}
		p.config.Operation.RetainCleanup(p.closeErr)
	})
}

func (p *Publisher) joinSourceUsers() error {
	<-p.sourceUsersDone
	return p.sourceUsersErr
}
