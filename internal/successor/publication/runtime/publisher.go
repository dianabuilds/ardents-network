package runtime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection"
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
	Responder        prefix.Config
	PrepareResponder func(context.Context) (prefix.Config, error)
	PrepareSource    func(context.Context) (prefix.Config, error)
	// PrepareJoin may fund only the accepted Rendezvous through this exact
	// existing Source. It joins its own work before returning, grants no
	// physical opening and cannot replace the Source or extend the caller.
	PrepareJoin func(context.Context, *prefix.SourceHandle, network.RetainedDuty) error
	Duty        network.RetainedDuty
	Deadline    time.Time
	Exclusions  []route.Member
}

type Publisher struct {
	self                          *Publisher
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
	cleanupErr                    error
	cleanupJoined                 atomic.Bool
	failure                       error
	watcherDone, schedulerDone    chan struct{}
	wake                          chan struct{}
	responderScope                *prefix.ResponderScope
	sourceScope                   *prefix.SourceScope
	initialDeliveries             map[[32]byte]*initialDelivery
	initialUsers                  sync.WaitGroup
	initialWake                   chan struct{}
	connections                   map[[32]byte]*connection.Stream
	connectionUsers               sync.WaitGroup
}

func New(ctx context.Context, config Config) (*Publisher, error) {
	if ctx == nil || config.Operation == nil || config.Binding == nil || config.Source == nil || config.Introduction == nil || config.Source == config.Introduction || config.Responder.Release == nil || config.Responder.Current == nil || config.Responder.Present == nil || config.PrepareResponder == nil || config.PrepareSource == nil || config.Responder.Leg.EntryMember.RoleDomain != 3 || config.Deadline != config.Deadline.UTC().Truncate(time.Second) || !time.Now().Before(config.Deadline) {
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
		return nil, errors.Join(err, lifetime.Close())
	}
	if bound := credential.Delegation().NotAfter; bound.Before(config.Deadline) {
		config.Deadline = bound
	}
	// Local stop seals Publication and interrupts its users. Physical parents
	// retain the original caller and deadline through their bounded retirement;
	// original caller/Operation/Instance loss still interrupts both lifetimes.
	physicalContext, cancelPhysical := context.WithDeadline(ctx, config.Deadline)
	child, cancel := context.WithCancel(physicalContext)
	p := &Publisher{config: config, lifetime: lifetime, credential: credential, administration: administration, history: history, ctx: child, cancel: cancel, physicalContext: physicalContext, cancelPhysical: cancelPhysical, lifetimeCallback: make(chan struct{}), operationCallback: make(chan struct{}), watcherDone: make(chan struct{}), schedulerDone: make(chan struct{}), wake: make(chan struct{}, 1)}
	p.self = p
	p.config.Exclusions = append([]route.Member(nil), config.Exclusions...)
	p.sourceScope, err = prefix.NewSourceScope(config.Operation.Context(), config.Source, config.PrepareSource)
	if err != nil {
		cancel()
		cancelPhysical()
		return nil, errors.Join(err, lifetime.Close())
	}
	p.responderScope, err = prefix.NewResponderScope(physicalContext, p.sourceScope, config.Responder, config.PrepareResponder)
	if err == nil {
		err = p.sourceScope.Commit(func(checkOriginal func() error) error {
			return errors.Join(ctx.Err(), child.Err(), lifetime.Context().Err(), checkOriginal(), config.Operation.CheckPublisher())
		})
	}
	if err != nil {
		responderErr := p.responderScope.Close()
		sourceErr := p.sourceScope.Close()
		responderCleanup, _ := p.responderScope.CleanupResult()
		sourceCleanup, sourceJoined := p.sourceScope.CleanupResult()
		if !sourceJoined {
			sourceCleanup = errors.Join(sourceCleanup, errors.New("rejected Publisher Source scope did not join"))
		}
		cancel()
		cancelPhysical()
		lifetimeErr := lifetime.Close()
		config.Operation.RetainCleanup(errors.Join(responderCleanup, sourceCleanup, lifetimeErr))
		return nil, errors.Join(err, responderErr, sourceErr, lifetimeErr)
	}
	p.initialDeliveries = make(map[[32]byte]*initialDelivery)
	p.initialWake = make(chan struct{}, 1)
	p.connections = make(map[[32]byte]*connection.Stream)
	p.stopLifetime = context.AfterFunc(lifetime.Context(), func() { defer close(p.lifetimeCallback); cancelPhysical() })
	p.stopOperation = context.AfterFunc(config.Operation.Context(), func() { defer close(p.operationCallback); cancelPhysical() })
	go func() {
		defer close(p.watcherDone)
		<-child.Done()
		p.stop()
		p.drain()
	}()
	go p.schedule()
	return p, nil
}

func (p *Publisher) check(ctx context.Context) error {
	if p == nil || p.self != p || ctx == nil {
		return errors.New("Publisher caller absent")
	}
	_, err := p.lifetime.Credential()
	return errors.Join(ctx.Err(), p.ctx.Err(), p.administration.Check(p.config.Operation), err, p.sourceScope.Check())
}

func (p *Publisher) checkLocal(ctx context.Context) error {
	if p == nil || p.self != p || ctx == nil {
		return errors.New("original Publisher unavailable")
	}
	return errors.Join(ctx.Err(), p.ctx.Err(), p.administration.Check(p.config.Operation), p.lifetime.CheckLocal(ctx))
}

// CurrentSource lends only the exact existing physical generation for issuing
// through this original Publisher. It cannot open, replace or close the scope.
func (p *Publisher) CurrentSource(ctx context.Context) (*prefix.SourceHandle, error) {
	if p == nil || p.self != p {
		return nil, errors.New("Publisher Source absent")
	}
	if err := p.check(ctx); err != nil {
		return nil, err
	}
	return p.sourceScope.CurrentHandle(ctx)
}

// Link is visible only under the current exact acknowledged accepting pair.
func (p *Publisher) Link(ctx context.Context) (string, error) {
	if p == nil || p.self != p {
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
	var link string
	err := p.sourceScope.Commit(func(checkSource func() error) error {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.closed || p.current != pair || pair.firstACK.IsZero() {
			return errors.New("Publisher accepting pair changed")
		}
		if err := errors.Join(checkSource(), p.checkLocal(ctx)); err != nil {
			return err
		}
		value := p.credential.Delegation()
		var err error
		link, err = publication.TargetLink(value.Network, value.Target)
		return err
	})
	return link, err
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
	for _, stream := range p.connections {
		stream.Stop()
	}
	p.cancel()
	p.mu.Unlock()
}

func (p *Publisher) Close() error {
	if p == nil {
		return nil
	}
	if p.self != p {
		return errors.New("copied or detached Publisher")
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
		p.joinConnections()
		p.mu.Lock()
		p.closeErr = p.failure
		physical := p.cleanupErr
		p.mu.Unlock()
		// No producer remains. Physical registrations/prefixes join before any
		// private recipient is erased or the Instance reservation is returned.
		for _, pair := range []*registrationPair{p.pending, p.current, p.predecessor} {
			if pair != nil {
				p.closeErr = errors.Join(p.closeErr, pair.close())
				physical = errors.Join(physical, pair.cleanupErr)
			}
		}
		// TLS borrowers join before returning either original Prefix claim.
		// Their Instance key remains retained through actual physical cleanup.
		// Admission and registration users are already joined. Detach the
		// external lifetime-loss callback before our own lifetime retirement:
		// that intentional cancellation must not abort graceful Prefix cleanup.
		// If external loss already started, join its original interruption.
		if !p.stopLifetime() {
			<-p.lifetimeCallback
		}
		lifetimeCleanup := p.lifetime.Close()
		p.closeErr = errors.Join(p.closeErr, lifetimeCleanup)
		physical = errors.Join(physical, lifetimeCleanup)
		// Private/Connection users are joined. Retire exact Responder
		// generations before their retained Source scope and Instance binding.
		responderWork := p.responderScope.Close()
		responderCleanup, responderJoined := p.responderScope.CleanupResult()
		if !responderJoined {
			responderCleanup = errors.Join(responderCleanup, errors.New("Publisher Responder scope did not join"))
		}
		p.closeErr = errors.Join(p.closeErr, responderWork)
		physical = errors.Join(physical, responderCleanup)
		p.closeErr = errors.Join(p.closeErr, p.config.Introduction.Close(), p.sourceScope.Close())
		for _, original := range []*prefix.Prefix{p.config.Introduction} {
			cleanup, complete := original.CleanupResult()
			if !complete {
				cleanup = errors.Join(cleanup, errors.New("original Publisher Prefix cleanup did not join"))
			}
			physical = errors.Join(physical, cleanup)
		}
		sourceCleanup, sourceJoined := p.sourceScope.CleanupResult()
		if !sourceJoined {
			sourceCleanup = errors.Join(sourceCleanup, errors.New("Publisher Source scope did not join"))
		}
		physical = errors.Join(physical, sourceCleanup)
		p.cancelPhysical()
		bindingCleanup := p.config.Binding.Close()
		p.closeErr = errors.Join(p.closeErr, bindingCleanup)
		physical = errors.Join(physical, bindingCleanup)
		if !p.stopOperation() {
			<-p.operationCallback
		}
		p.cleanupErr = physical
		p.config.Operation.RetainCleanup(physical)
		p.cleanupJoined.Store(true)
	})
}

// CleanupResult reports this original Publisher's physically joined borrowers,
// Prefixes and private binding. Full Publication/refusal results remain with
// Close; this receipt grants neither accepting readiness nor another Job.
func (p *Publisher) CleanupResult() (error, bool) {
	if p == nil || p.self != p || !p.cleanupJoined.Load() {
		return nil, false
	}
	select {
	case <-p.watcherDone:
		return p.cleanupErr, true
	default:
		return nil, false
	}
}
