package prefix

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/bootstrap"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	rolequic "github.com/dianabuilds/ardents-network/internal/successor/route/transport/quic"
	roletls "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls"
)

// Config retains the selected leg and absolute bounds. Present performs
// genuine Stock presentation. Release returns the local Hosting reservation
// only after this owner has joined the complete physical tree.
type Config struct {
	Leg        selection.Leg
	Current    func() (network.RuntimeView, error)
	Present    role.Presentation
	Deadline   time.Time
	Release    func() error
	HoldRefill func(context.Context, uint64) (func() error, error)
}

// Prefix is a bounded Entry/Interior transport. It carries no Service data or
// fake terminal workload. Its accepted result means both exact roles admitted
// fresh authenticated channels; Close retains one joined terminal result.
type Prefix struct {
	bootstrap                 bool
	queues                    *framing.Budget
	entry, interior           *framing.Session
	entryHello, interiorHello ardp.Hello
	refills                   map[*prefixRefill]struct{}
	refillReturns             []func() error
	child                     *framing.Lane
	ctx                       context.Context
	caller                    context.Context
	cancel                    context.CancelFunc
	done                      chan struct{}
	closing                   chan struct{}
	once                      sync.Once
	sealOnce                  sync.Once
	err                       error
	release                   func() error
	config                    Config
	lifetimeMu                sync.Mutex
	borrows                   map[*Borrow]struct{}
	setups                    map[*prefixOpening]struct{}
	openings                  sync.WaitGroup
	activity                  chan struct{}
	source                    *Prefix
	responders                map[*Prefix]struct{}
	stopSource                func() bool
	sourceStopped             chan struct{}
	openingConn               net.Conn
	openingErr                error
}

// A failed opening can be retried only if its original physical retirement was
// clean. Preserve that provenance without changing either error's identity.
type prefixOpeningFailure struct {
	operation, retirement error
}

func (e *prefixOpeningFailure) Error() string   { return errors.Join(e.operation, e.retirement).Error() }
func (e *prefixOpeningFailure) Unwrap() []error { return []error{e.operation, e.retirement} }

func Open(ctx context.Context, config Config) (_ *Prefix, result error) {
	return openPrefix(ctx, config, nil, false)
}

// OpenResponder retains the actual Source generation before any physical
// opening. The resulting Responder cannot acquire JOIN using another Source.
func OpenResponder(ctx context.Context, source *Prefix, config Config) (*Prefix, error) {
	if source == nil || source.bootstrap || source.config.Leg.EntryMember.RoleDomain != 1 || config.Leg.EntryMember.RoleDomain != 3 {
		return nil, errors.New("route Responder original Source unavailable")
	}
	return openPrefix(ctx, config, source, false)
}

func openPrefix(ctx context.Context, config Config, source *Prefix, bootstrapMode bool) (_ *Prefix, result error) {
	lifetime := admission.ForwardClass.Lifetime()
	if bootstrapMode {
		lifetime = bootstrap.Lifetime
	}
	if ctx == nil || config.Current == nil || !bootstrapMode && config.Present == nil || config.Release == nil || config.Deadline.IsZero() || config.Deadline != config.Deadline.UTC().Truncate(time.Second) ||
		!time.Now().Before(config.Deadline) || config.Deadline.After(config.Leg.NotAfter) || config.Deadline.After(time.Now().Add(lifetime)) || bootstrapMode && (source != nil || config.Leg.EntryMember.RoleDomain != 1 || config.Present != nil) {
		return nil, errors.New("route prefix composition or deadline invalid")
	}
	childContext, cancel := context.WithDeadline(ctx, config.Deadline)
	if source != nil {
		opening, err := source.beginOpening(cancel)
		if err != nil {
			cancel()
			return nil, err
		}
		defer opening.finish()
		observe := config.Current
		config.Current = func() (network.RuntimeView, error) {
			if err := source.originalCurrent(); err != nil {
				return network.RuntimeView{}, err
			}
			view, err := observe()
			if err != nil {
				return network.RuntimeView{}, err
			}
			if err := source.originalCurrent(); err != nil {
				return network.RuntimeView{}, err
			}
			return view, nil
		}
	}
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		v, err := config.Current()
		if err != nil {
			return err
		}
		// An observation may finish after the original caller was revoked.
		// Derived cancellation callbacks need not have run at this boundary.
		return errors.Join(config.Leg.Check(v, time.Now()), ctx.Err())
	}
	if err := check(); err != nil {
		cancel()
		return nil, err
	}
	p := &Prefix{bootstrap: bootstrapMode, ctx: childContext, caller: ctx, cancel: cancel, done: make(chan struct{}), closing: make(chan struct{}), release: config.Release, config: config, activity: make(chan struct{}, 1), source: source}
	if source != nil {
		p.sourceStopped = make(chan struct{})
		p.stopSource = context.AfterFunc(source.ctx, func() { defer close(p.sourceStopped); cancel() })
	}
	// The local reservation transfers at this point, including failed setup.
	defer func() {
		if result != nil {
			if retirement := p.closeOpening(); retirement != nil {
				result = &prefixOpeningFailure{operation: result, retirement: retirement}
			}
		}
	}()
	queueBytes, allowance := uint64(4<<20), admission.ForwardClass.ByteLimit()-role.AdmissionWireBytes
	if bootstrapMode {
		queueBytes, allowance = 256<<10, bootstrap.LaneBytes-(3*ardp.HeaderSize+209+1+5)
	}
	queues := framing.NewBudget(queueBytes)
	p.queues = queues
	entryControl, err := queues.HoldControl()
	if err != nil {
		return nil, err
	}
	interiorControl, err := queues.HoldControl()
	if err != nil {
		entryControl()
		return nil, err
	}
	p.release = func() error { entryControl(); interiorControl(); return config.Release() }
	a := role.Authority{Current: config.Current, Duty: config.Leg.Entry, Profile: config.Leg.Profile}
	m, err := a.Member()
	if err != nil {
		return nil, err
	}
	request := transport.ClosedRoleCarrierRequest{CarrierProfile: transport.CarrierProfile(m.CarrierProfile), Endpoint: m.Endpoint, ExpectedServer: m.PublicKey, Deadline: minDeadline(config.Deadline, time.Now().Add(10*time.Second))}
	if err := transport.ValidateRoleRequest(childContext, request); err != nil {
		return nil, err
	}
	var conn net.Conn
	switch request.CarrierProfile {
	case transport.ClosedCarrierTCP:
		conn, err = roletls.OpenEndpoint(childContext, request)
	case transport.ClosedCarrierQUIC:
		conn, err = rolequic.OpenEndpoint(childContext, request)
	default:
		err = errors.New("closed role carrier profile is unsupported")
	}
	if err != nil {
		return nil, err
	}
	conn = transport.Retain(conn)
	p.openingConn = conn
	// Cancellation interrupts setup even before a session reader exists.
	stopSetup := interruptPrefixOpening(childContext, conn)
	defer func() { p.openingErr = stopSetup() }()
	if err := conn.SetDeadline(minDeadline(config.Deadline, time.Now().Add(10*time.Second))); err != nil {
		_ = conn.Close()
		return nil, err
	}
	h, err := a.FreshHello(config.Deadline, ardp.PurposeForwarding, false)
	if err == nil {
		err = presentPrefixRole(childContext, ctx, conn, a, h, config.Present, bootstrapMode)
	}
	if err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	if err := check(); err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	if err := conn.SetDeadline(config.Deadline); err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	p.entry = framing.New(childContext, conn, config.Deadline, allowance, check, false, queues, nil)
	p.entryHello = h
	p.openingConn = nil
	opened := ardp.Open{RecipientNodeID: config.Leg.InteriorMember.NodeID, RecipientDutyGeneration: config.Leg.InteriorMember.DutyGeneration, Purpose: 7, Deadline: config.Deadline}
	p.child, err = p.entry.Open(childContext, ctx, ardp.EncodeOpen(opened, false))
	if err != nil {
		return nil, err
	}
	a = role.Authority{Current: config.Current, Duty: config.Leg.Interior, Profile: config.Leg.Profile}
	m, err = a.Member()
	if err != nil {
		return nil, err
	}
	secured, err := roletls.OpenRole(childContext, p.child, m.PublicKey, minDeadline(config.Deadline, time.Now().Add(10*time.Second)))
	if err != nil {
		return nil, err
	}
	h, err = a.FreshHello(config.Deadline, ardp.PurposeForwarding, false)
	if err == nil {
		err = presentPrefixRole(childContext, ctx, secured, a, h, config.Present, bootstrapMode)
	}
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := childContext.Err(); err != nil {
		return nil, err
	}
	if err := secured.SetDeadline(config.Deadline); err != nil {
		return nil, err
	}
	p.interior = framing.New(childContext, transport.Retain(secured), config.Deadline, allowance, check, false, queues, nil)
	p.interiorHello = h
	if err := check(); err != nil {
		return nil, err
	}
	if err := childContext.Err(); err != nil {
		return nil, err
	}
	// The setup callback must relinquish this connection before publication;
	// a late setup cancellation cannot close an already transferred prefix.
	if err := errors.Join(stopSetup(), ctx.Err(), childContext.Err()); err != nil {
		return nil, err
	}
	if source != nil {
		source.lifetimeMu.Lock()
		defer source.lifetimeMu.Unlock()
		if err := errors.Join(source.localCurrent(), ctx.Err(), childContext.Err()); err != nil {
			return nil, err
		}
		if source.responders == nil {
			source.responders = make(map[*Prefix]struct{})
		}
		source.responders[p] = struct{}{}
	}
	go p.watch(check, nil)
	return p, nil
}

func (p *Prefix) watch(check func() error, idleEvents <-chan time.Time) {
	defer close(p.done)
	idle := time.NewTimer(120 * time.Second)
	defer idle.Stop()
	if idleEvents == nil {
		idleEvents = idle.C
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-p.activity:
			idle.Reset(120 * time.Second)
		case <-idleEvents:
			p.lifetimeMu.Lock()
			busy := len(p.setups) != 0 || len(p.refills) != 0
			for borrow := range p.borrows {
				busy = busy || borrow.active()
			}
			if busy {
				p.lifetimeMu.Unlock()
				idle.Reset(120 * time.Second)
				continue
			}
			// Idle expiration revokes readiness; the composing owner then
			// joins this generation, including its dependent Responder,
			// before returning capacity. Keep original parents live for
			// bounded terminal output and retain actual cleanup failures.
			p.sealLocked()
			p.lifetimeMu.Unlock()
			p.once.Do(func() { p.err = p.closeOpening() })
			return
		case <-p.ctx.Done():
			select {
			case <-p.closing:
				return
			default:
			}
			p.interior.Retire(p.ctx.Err())
			p.entry.Retire(p.ctx.Err())
			return
		case <-p.entry.Done():
			// Close already sealed this generation and owns ordered parent
			// retirement. Its first reader joining must not cancel the
			// other original parent before that owner's clean Retire.
			select {
			case <-p.closing:
				return
			default:
			}
			p.cancel()
			return
		case <-p.interior.Done():
			select {
			case <-p.closing:
				return
			default:
			}
			p.cancel()
			return
		case <-ticker.C:
			if err := check(); err != nil {
				p.entry.Retire(err)
				p.cancel()
				return
			}
		}
	}
}

func (p *Prefix) closeOpening() error {
	p.lifetimeMu.Lock()
	for refill := range p.refills {
		refill.cancel()
	}
	responders := make([]*Prefix, 0, len(p.responders))
	for responder := range p.responders {
		responders = append(responders, responder)
		responder.Seal()
	}
	borrows := make([]*Borrow, 0, len(p.borrows))
	for borrow := range p.borrows {
		borrows = append(borrows, borrow)
		borrow.interrupt()
	}
	for opening := range p.setups {
		opening.cancel()
	}
	p.lifetimeMu.Unlock()
	// Terminal borrowers need their still-live framing parents to emit their
	// bounded CLOSE and join. Seal/cancel every terminal acquisition first;
	// only retire the parents after those original borrowers have joined.
	p.openings.Wait()
	joined := p.openingErr
	// A Source retains its published dependents as well as setup reservations.
	// Their original authority remains live until their terminal I/O joins.
	for _, responder := range responders {
		joined = errors.Join(joined, responder.Close())
	}
	if p.openingConn != nil {
		joined = errors.Join(joined, p.openingConn.Close())
	}
	for _, borrow := range borrows {
		joined = errors.Join(joined, borrow.join())
	}
	// Retire the inner reader before interrupting its lower framing owner, so
	// our own lower closure cannot become an unexplained inner transport EOF.
	if p.interior != nil {
		if p.interiorHello != (ardp.Hello{}) && p.interior.Live() && p.ctx.Err() == nil && p.caller.Err() == nil {
			// A healthy explicit/idle close completes the original inner role
			// before its lower Carrier is interrupted. Revocation retains the
			// immediate physical retirement path below.
			joined = errors.Join(joined, p.interior.FinishRole())
		}
		p.interior.Retire(nil)
	}
	if p.entry != nil {
		p.entry.Retire(nil)
	}
	p.cancel()
	if p.stopSource != nil && !p.stopSource() {
		<-p.sourceStopped
	}
	if p.interior != nil {
		if err := p.interior.Close(); err != nil {
			joined = errors.Join(joined, fmt.Errorf("prefix Interior: %w", err))
		}
	}
	if p.entry != nil {
		if err := p.entry.Close(); err != nil {
			joined = errors.Join(joined, fmt.Errorf("prefix Entry: %w", err))
		}
	}
	if p.release != nil {
		for i := len(p.refillReturns) - 1; i >= 0; i-- {
			joined = errors.Join(joined, p.refillReturns[i]())
		}
		joined = errors.Join(joined, p.release())
	}
	if p.source != nil {
		p.source.lifetimeMu.Lock()
		delete(p.source.responders, p)
		p.source.lifetimeMu.Unlock()
	}
	return joined
}

// Done closes when retained readiness retires. It is not a completed physical
// join: the work owner must call Close before releasing its other roots.
func (p *Prefix) Done() <-chan struct{} { return p.done }

// Seal synchronously denies new children and signals every current terminal
// borrower before a composing owner starts joining any sibling prefix.
func (p *Prefix) Seal() {
	if p == nil {
		return
	}
	p.lifetimeMu.Lock()
	defer p.lifetimeMu.Unlock()
	p.sealLocked()
}

func (p *Prefix) sealLocked() {
	p.sealOnce.Do(func() {
		close(p.closing)
		for refill := range p.refills {
			refill.cancel()
		}
		for opening := range p.setups {
			opening.cancel()
		}
		for borrow := range p.borrows {
			borrow.interrupt()
		}
	})
}

func (p *Prefix) Close() error {
	if p == nil {
		return nil
	}
	p.once.Do(func() {
		p.Seal()
		p.err = p.closeOpening()
	})
	<-p.done
	return p.err
}

var _ net.Conn = (*framing.Lane)(nil)

// OpeningRetirement extracts only the physical retirement of a failed opening.
// Ordinary setup refusal with clean retirement grants no terminal failure.
func OpeningRetirement(err error) error {
	var failed *prefixOpeningFailure
	if errors.As(err, &failed) {
		return failed.retirement
	}
	return nil
}
