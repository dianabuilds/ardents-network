//go:build linux

package receiver

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/bootstrap"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
	issuertransport "github.com/dianabuilds/ardents-network/internal/successor/route/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	rolequic "github.com/dianabuilds/ardents-network/internal/successor/route/transport/quic"
	roletls "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls"
)

// ReceiverConfig binds one actual new receiving forwarding duty. Admit must
// call receiving.Owner.Accept with its genuine Hosting capacity callback.
type ReceiverConfig struct {
	Authority   role.Authority
	Certificate tls.Certificate
	Admit       Admit
	Refill      Refill
	// ReserveBootstrap reserves actual Hosting work and termination before
	// pending child TLS. Its return is retained through physical lane finish.
	// It supplies resources only, never Admission or issuer authority.
	ReserveBootstrap func(context.Context, time.Time) (func() error, error)
	// Issue calls the genuine issuing owner with the exact canonical batch.
	// The transport derives bootstrap from its immutable incoming claim.
	Issue            func(context.Context, []byte, bool) ([]byte, error)
	IntroductionRoot string
	Receiving        *receiving.Owner
	// DescriptorStore is the genuine separately owned receiving Store. Command
	// composition closes its root only after this Receiver has physically joined.
	DescriptorStore *reachability.Store
}

// Receiver owns its listener, in-flight handshakes, admitted parents and all
// forwarding descendants. Admission/spend roots are closed by composition only
// after Close returns its retained joined result.
type Receiver struct {
	config        ReceiverConfig
	listener      transport.ClosedSharedCarrierListener
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.Mutex
	connections   map[net.Conn]struct{}
	workers       sync.WaitGroup
	done          chan struct{}
	once          sync.Once
	listenerOnce  sync.Once
	listenerErr   error
	queues        *framing.Budget
	bootstrap     *bootstrap.Budget
	pool          nodePool
	err, closeErr error
	registrations *introduction.Registry
	joins         *join.Pairing
	issuerWork    chan struct{}
}

func Listen(ctx context.Context, config ReceiverConfig) (*Receiver, error) {
	if ctx == nil || config.Admit == nil {
		return nil, errors.New("route receiving composition absent")
	}
	m, err := config.Authority.Member()
	if err != nil {
		return nil, err
	}
	key, ok := config.Certificate.PrivateKey.(ed25519.PrivateKey)
	forwarding := (m.RoleDomain == 1 || m.RoleDomain == 3 || m.RoleDomain == 4) && (m.Subrole == 1 || m.Subrole == 2)
	introductionDuty := m.RoleDomain == 4 && m.Subrole == 3
	joinDuty := m.RoleDomain == 2 && m.Subrole == 4
	issuerDuty := m.RoleDomain == 2 && m.Subrole == 6
	resolutionDuty := m.RoleDomain == 2 && m.Subrole == 5
	if !ok || len(key) != ed25519.PrivateKeySize || string(key.Public().(ed25519.PublicKey)) != string(m.PublicKey[:]) || (!forwarding && !introductionDuty && !joinDuty && !issuerDuty && !resolutionDuty) || (issuerDuty && config.Issue == nil) || (resolutionDuty != (config.DescriptorStore != nil)) {
		return nil, errors.New("route receiving duty or private key differs")
	}
	child, cancel := context.WithCancel(ctx)
	r := &Receiver{config: config, ctx: child, cancel: cancel, connections: make(map[net.Conn]struct{}), done: make(chan struct{}), queues: framing.NewBudget(64 << 20), bootstrap: bootstrap.NewBudget()}
	r.joins = join.NewPairing(r.record)
	if issuerDuty {
		r.issuerWork = make(chan struct{}, 1)
	}
	if m.Subrole == 3 {
		if config.IntroductionRoot == "" || config.Receiving == nil {
			cancel()
			return nil, errors.New("introduction independent roots absent")
		}
		binding := introduction.Binding{NetworkID: config.Authority.Profile.Network, ProfileDigest: config.Authority.Profile.Digest, ReceiverNodeID: m.NodeID, ReceiverDutyGeneration: m.DutyGeneration}
		history, openErr := introduction.OpenHistory(config.IntroductionRoot, binding)
		if openErr != nil {
			names, readErr := os.ReadDir(config.IntroductionRoot)
			if readErr != nil || len(names) != 0 {
				cancel()
				return nil, errors.Join(openErr, readErr)
			}
			fact, factErr := config.Receiving.TakeFreshRoot()
			if factErr != nil {
				cancel()
				return nil, errors.Join(openErr, factErr)
			}
			history, openErr = introduction.InitializeHistory(config.IntroductionRoot, binding, fact)
		}
		if openErr != nil {
			cancel()
			return nil, openErr
		}
		r.registrations, err = introduction.NewRegistry(history)
		if err != nil {
			cancel()
			return nil, errors.Join(err, history.Close())
		}
	} else if config.IntroductionRoot != "" || config.Receiving != nil {
		cancel()
		return nil, errors.New("introduction history supplied to another duty")
	}
	verify := func(key [32]byte) bool {
		_, err := r.peer(key)
		return err == nil
	}
	err = transport.ValidateSharedListener(m.Endpoint, config.Certificate, verify, 16)
	if err == nil {
		switch transport.CarrierProfile(m.CarrierProfile) {
		case transport.ClosedCarrierTCP:
			r.listener, err = roletls.ListenShared(m.Endpoint, config.Certificate, verify, 16)
		case transport.ClosedCarrierQUIC:
			r.listener, err = rolequic.ListenShared(m.Endpoint, config.Certificate, verify, 16)
		default:
			err = errors.New("closed shared carrier profile is unsupported")
		}
	}
	if err != nil {
		cancel()
		return nil, errors.Join(err, r.registrations.Close())
	}
	go r.run()
	return r, nil
}

func (r *Receiver) peer(key [32]byte) (network.Member, error) {
	m, err := r.config.Authority.Member()
	if err != nil {
		return network.Member{}, err
	}
	v, err := r.config.Authority.Current()
	if err != nil {
		return network.Member{}, err
	}
	peer, err := v.MemberByKey(key, time.Now())
	expectedSubrole := uint8(1)
	allowedDomain := peer.RoleDomain == m.RoleDomain
	if m.Subrole == 3 {
		expectedSubrole = 2
		if m.RoleDomain == 4 {
			allowedDomain = peer.RoleDomain == 1 || peer.RoleDomain == 4
		}
	}
	if m.RoleDomain == 2 && m.Subrole == 4 {
		expectedSubrole = 2
		allowedDomain = peer.RoleDomain == 1 || peer.RoleDomain == 3
	}
	if m.RoleDomain == 2 && (m.Subrole == 5 || m.Subrole == 6) {
		expectedSubrole = 2
		allowedDomain = peer.RoleDomain == 1
	}
	if err != nil || role.Conflicting(m, peer) || !allowedDomain || peer.Subrole != expectedSubrole {
		return network.Member{}, errors.Join(errors.New("route adjacent Node peer unavailable"), err)
	}
	return peer, nil
}

func (r *Receiver) run() {
	defer close(r.done)
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-r.ctx.Done():
				r.interrupt()
				return
			case <-ticker.C:
				if _, err := r.config.Authority.Member(); err != nil {
					r.record(err)
					r.cancel()
				}
			}
		}
	}()
	defer func() {
		r.cancel()
		r.interrupt()
		r.workers.Wait()
		r.record(r.pool.Close())
		<-watchDone
		r.record(r.registrations.Close())
	}()
	for {
		accepted, err := r.listener.Accept(r.ctx, 10*time.Second)
		if err != nil {
			if transport.IsClosedSharedPeerFailure(err) {
				continue
			}
			if r.ctx.Err() == nil {
				r.record(err)
			}
			return
		}
		accepted.Connection = transport.Retain(accepted.Connection)
		r.mu.Lock()
		if r.ctx.Err() != nil || len(r.connections) >= 1024 {
			r.mu.Unlock()
			r.record(accepted.Connection.Close())
			continue
		}
		r.connections[accepted.Connection] = struct{}{}
		r.workers.Add(1)
		r.mu.Unlock()
		go func() {
			defer r.workers.Done()
			conn := accepted.Connection
			defer func() { r.record(conn.Close()); r.mu.Lock(); delete(r.connections, conn); r.mu.Unlock() }()
			if accepted.Kind == transport.ClosedSharedDirect {
				m, err := r.config.Authority.Member()
				if err == nil && m.Subrole == 1 {
					_ = r.serveRole(r.ctx, conn, nil, nil, nil)
				}
			} else {
				_ = r.serveOuter(r.ctx, accepted)
			}
		}()
	}
}

func (r *Receiver) record(err error) {
	if err != nil {
		r.mu.Lock()
		r.err = errors.Join(r.err, err)
		r.mu.Unlock()
	}
}
func (r *Receiver) interrupt() {
	// The generation is already canceled, so run cannot publish another accepted
	// borrower. Interrupt accepted connections while the shared transport still
	// exists: QUIC Transport.Close otherwise destroys them without sending their
	// connection-close packet, leaving peers to discover retirement by timeout.
	r.pool.interrupt()
	r.mu.Lock()
	for conn := range r.connections {
		_ = conn.SetDeadline(time.Now())
		_ = conn.Close()
	}
	r.mu.Unlock()
	// Listener close now interrupts unaccepted handshakes and releases its shared
	// socket. Worker/resource join remains the responsibility of run and Close.
	_ = r.closeListener()
}

// Done signals joined listener retirement. Close retains the terminal result.
func (r *Receiver) Done() <-chan struct{} { return r.done }
func (r *Receiver) Close() error {
	if r == nil {
		return nil
	}
	r.once.Do(func() {
		r.cancel()
		r.interrupt()
		<-r.done
		r.mu.Lock()
		r.closeErr = errors.Join(r.err, r.closeListener())
		r.mu.Unlock()
	})
	return r.closeErr
}

func (r *Receiver) serveOuter(ctx context.Context, accepted transport.ClosedSharedCarrier) (result error) {
	releaseControl, err := r.queues.HoldControl()
	if err != nil {
		return err
	}
	defer releaseControl()
	conn := accepted.Connection
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	h, err := framing.ReadHello(conn)
	if err != nil {
		return err
	}
	member, err := r.config.Authority.Hello(h, true)
	if err != nil {
		return err
	}
	peer, err := r.peer(accepted.NodeKey)
	if err != nil {
		return err
	}
	if err := framing.Accept(conn); err != nil {
		return err
	}
	if err := conn.SetDeadline(h.Deadline); err != nil {
		return err
	}
	check := func() error {
		if _, err := r.config.Authority.Hello(h, true); err != nil {
			return err
		}
		current, err := r.peer(accepted.NodeKey)
		if err != nil || current.RecordDigest != peer.RecordDigest || current.DutyGeneration != peer.DutyGeneration {
			return errors.Join(errors.New("route Node peer changed"), err)
		}
		return nil
	}
	adjacency := r.bootstrap.Adjacency()
	defer adjacency.Seal()
	// Preparation is synchronous; Open owns the later physical work. A
	// handler's Finish and parent Close join that work before returning its
	// reservation. The map transfers only that exact lane's resource lifetime.
	type bootstrapWork struct {
		claim    *bootstrap.Claim
		terminal *bootstrap.Termination
		release  func() error
	}
	var bootstrapChildren sync.Map
	s := framing.Prepare(ctx, conn, h.Deadline, 32<<20, check, true, r.queues, framing.Handlers{PrepareOpen: func(l *framing.Lane, body []byte) error {
		nodeOpen, err := ardp.DecodeNodeOpen(body)
		if err != nil {
			return errors.New("route Node OPEN restriction unavailable")
		}
		var claim *bootstrap.Claim
		var bootstrapState *bootstrapWork
		if nodeOpen.Restriction == ardp.IssuerBootstrapChild {
			terminal, terminalErr := r.bootstrap.HoldTermination(l.Deadline())
			work := &bootstrapWork{terminal: terminal}
			bootstrapState = work
			if terminal != nil {
				if err := l.RetainUntilFinish(func() {
					bootstrapChildren.Delete(l)
					if work.release != nil {
						r.record(work.release())
					}
					work.terminal.ReleaseAfterJoin()
					claim.ReleaseAfterJoin()
				}); err != nil {
					terminal.ReleaseAfterJoin()
					return err
				}
			}
			if err := l.ConstrainTraffic(bootstrap.LaneBytes, func(cost uint64, closing bool) error {
				if closing {
					if cost != ardp.HeaderSize+1 || terminal == nil {
						return errors.Join(errors.New("bootstrap terminal output unavailable"), terminalErr)
					}
					return terminal.Emit()
				}
				return claim.ChargeOutput(cost)
			}); err != nil {
				return err
			}
			if terminalErr != nil {
				return terminalErr
			}
			if err := l.ConstrainQueues(r.bootstrap.Queues()); err != nil {
				return err
			}
			bootstrapChildren.Store(l, work)
		}
		opened := nodeOpen.Recipient
		if !time.Now().Before(opened.Deadline) {
			return errors.New("route OPEN facts invalid")
		}
		if opened.RecipientNodeID != h.RecipientNodeID || opened.RecipientDutyGeneration != h.RecipientDutyGeneration || opened.Deadline.After(h.Deadline) {
			return errors.New("route Node child binding differs")
		}
		if err := l.Bound(opened.Deadline); err != nil {
			return err
		}
		// The frame check reobserves this same immutable outer binding.
		// Capacity preparation adds no storage observation to the sole reader.
		m := member
		if m.RoleDomain == 4 && m.Subrole == 3 && !introductionChildPermitted(peer.RoleDomain, opened.Purpose) {
			return errors.New("route Introduction child purpose differs from adjacent Node")
		}
		if !((m.Subrole == 2 && opened.Purpose == 7) || (m.RoleDomain == 4 && m.Subrole == 3 && (opened.Purpose == 4 || opened.Purpose == 5)) || (m.RoleDomain == 2 && m.Subrole == 4 && opened.Purpose == 6) || (m.RoleDomain == 2 && m.Subrole == 5 && opened.Purpose == 3) || (m.RoleDomain == 2 && m.Subrole == 6 && opened.Purpose == 1)) {
			return errors.New("route Interior child unavailable")
		}
		if nodeOpen.Restriction == ardp.IssuerBootstrapChild {
			if !(m.RoleDomain == 1 && m.Subrole == 2 && opened.Purpose == uint8(ardp.PurposeForwarding)) && !(m.RoleDomain == 2 && m.Subrole == 6 && opened.Purpose == uint8(ardp.PurposeIssuer)) {
				return errors.New("bootstrap child duty unavailable")
			}
			claim, err = adjacency.Reserve(opened.Deadline)
			if err != nil {
				return err
			}
			bootstrapState.claim = claim
			if restriction, err := claim.Restriction(); err != nil || restriction != nodeOpen.Restriction {
				return errors.Join(errors.New("bootstrap child reservation unavailable"), err)
			}
			opened.Deadline = claim.Deadline()
			if err := l.Bound(opened.Deadline); err != nil {
				return err
			}
		}
		return nil
	}, Open: func(ctx context.Context, l *framing.Lane, body []byte) error {
		nodeOpen, err := ardp.DecodeNodeOpen(body)
		if err != nil {
			return err
		}
		opened := nodeOpen.Recipient
		_, err = r.config.Authority.Hello(h, true)
		if err = errors.Join(err, ctx.Err()); err != nil {
			return err
		}
		var bootstrapClaim *bootstrap.Claim
		if nodeOpen.Restriction == ardp.IssuerBootstrapChild {
			value, ok := bootstrapChildren.Load(l)
			if !ok || r.config.ReserveBootstrap == nil {
				return errors.New("bootstrap physical reservation unavailable")
			}
			work := value.(*bootstrapWork)
			bootstrapClaim = work.claim
			// No storage I/O runs on the sole framing reader. Even a late or
			// refused reservation return remains owned until this handler joins.
			work.release, err = r.config.ReserveBootstrap(ctx, l.Deadline())
			if err != nil || work.release == nil {
				return errors.Join(errors.New("bootstrap physical capacity unavailable"), err)
			}
			_, err = r.config.Authority.Hello(h, true)
			if err = errors.Join(err, ctx.Err()); err != nil {
				return err
			}
		}
		secured, err := roletls.AcceptRole(ctx, l, r.config.Certificate, l.Deadline())
		if err != nil {
			return err
		}
		if err := l.BeginRole(); err != nil {
			return errors.Join(err, secured.Close())
		}
		return r.serveRole(ctx, transport.Retain(secured), l, &opened, bootstrapClaim)
	}})
	s.Start()
	<-s.Done()
	return r.finishSession(s)
}

func (r *Receiver) finishSession(s *framing.Session) error {
	err := s.Close()
	if physical := s.PhysicalFailure(); physical != nil {
		r.record(fmt.Errorf("route receiving framing retirement: %w", physical))
	}
	return err
}

// The outer Carrier accepts both Introduction and Source Interiors, but each
// retains its own permitted child purpose before any role TLS or admission.
func introductionChildPermitted(parentDomain, purpose uint8) bool {
	return parentDomain == 4 && purpose == uint8(ardp.PurposeIntroduction) ||
		parentDomain == 1 && purpose == uint8(ardp.PurposeSubmission)
}

func (r *Receiver) serveRole(ctx context.Context, conn net.Conn, outer *framing.Lane, opened *ardp.Open, bootstrapClaim *bootstrap.Claim) (result error) {
	releaseControl, err := r.queues.HoldControl()
	if err != nil {
		return err
	}
	defer releaseControl()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	capacity := &admissionRetirement{registry: r.registrations, joinQueues: r.queues}
	start, first, err := readChannelStart(ctx, conn, r.config.Authority, opened, capacity)
	h := start.Hello
	var grant receiving.Grant
	var s *framing.Session
	var additions []receiving.Grant
	var releaseIssuer func()
	var releaseDescriptor func()
	defer func() {
		// Interrupt nested physical work, join every borrower, then return the
		// finite reservation. A retained token spend is never refunded here.
		if s != nil {
			result = errors.Join(result, r.finishSession(s))
		} else {
			result = errors.Join(result, conn.Close())
		}
		var additionalErr error
		for i := len(additions) - 1; i >= 0; i-- {
			additionalErr = errors.Join(additionalErr, additions[i].Release())
		}
		releaseErr := errors.Join(additionalErr, capacity.finish(), grant.Release())
		r.record(releaseErr)
		result = errors.Join(result, releaseErr)
		if releaseIssuer != nil {
			releaseIssuer()
		}
		if releaseDescriptor != nil {
			releaseDescriptor()
		}
	}()
	if err != nil {
		return err
	}
	if h.Purpose == ardp.PurposeIssuer {
		releaseIssuer, err = r.holdIssuer(bootstrapClaim != nil)
		if err != nil {
			clear(first.Body)
			return err
		}
	}
	if h.Purpose == ardp.PurposeReachability {
		releaseDescriptor, err = r.queues.HoldChild(6 * (ardp.HeaderSize + ardp.DescriptorResultBodySize))
		if err != nil {
			clear(first.Body)
			return err
		}
	}
	if first.Kind == ardp.KindBootstrap {
		defer clear(first.Body)
		if first.Body[0] != 2 {
			return errors.New("public evidence bootstrap unavailable")
		}
		return r.serveBootstrap(ctx, conn, outer, h, bootstrapClaim)
	}
	if bootstrapClaim != nil {
		clear(first.Body)
		return errors.New("restricted child refuses private Admission")
	}
	grant, err = receiveAdmission(ctx, r.config.Authority, r.config.Admit, start, first)
	if err != nil {
		return err
	}
	if opened != nil && (h.RecipientNodeID != opened.RecipientNodeID || h.RecipientDutyGeneration != opened.RecipientDutyGeneration || uint8(h.Purpose) != opened.Purpose || h.Deadline.After(opened.Deadline)) {
		return errors.New("route inner HELLO differs from OPEN")
	}
	if err := conn.SetDeadline(h.Deadline); err != nil {
		return err
	}
	if outer != nil {
		if err := outer.Admit(h.Deadline); err != nil {
			return err
		}
	}
	if err := framing.Accept(conn); err != nil {
		return err
	}
	check := func() error { _, err := r.config.Authority.Hello(h, false); return err }
	if err := check(); err != nil {
		return err
	}
	if h.Purpose == ardp.PurposeIssuer {
		return issuertransport.Serve(ctx, conn, h, grant.Allowance().Bytes()-role.AdmissionWireBytes, check, func(ctx context.Context, request []byte) ([]byte, error) { return r.config.Issue(ctx, request, false) })
	}
	if h.Purpose == ardp.PurposeReachability {
		return r.serveDescriptor(ctx, conn, h, grant.Allowance().Bytes()-role.AdmissionWireBytes, check)
	}
	if h.Purpose == ardp.PurposeIntroduction {
		if outer == nil {
			return errors.New("route Introduction original lane unavailable")
		}
		return r.registrations.ServeRegistration(ctx, conn, capacity.registration, introduction.RegistrationChannel{
			Hello: h, Authority: r.config.Authority, Bytes: grant.Allowance().Bytes(),
			Deadline: grant.Allowance().Deadline(), Record: r.record, InterruptIO: outer.InterruptIO,
		})
	}
	if h.Purpose == ardp.PurposeSubmission {
		return r.registrations.ServeSubmission(ctx, conn, introduction.RegistrationChannel{
			Hello: h, Authority: r.config.Authority, Bytes: grant.Allowance().Bytes(),
			Deadline: grant.Allowance().Deadline(), Record: r.record,
		})
	}
	if h.Purpose == ardp.PurposeDataJoin {
		return r.joins.Serve(ctx, conn, h, grant.Allowance().Bytes(), check, capacity.join)
	}
	handlers := framing.Handlers{Open: func(ctx context.Context, l *framing.Lane, body []byte) error {
		return r.forward(ctx, l, h, body, ardp.OrdinaryChild)
	}}
	if r.config.Refill != nil && h.Purpose == ardp.PurposeForwarding {
		hash, err := role.Binding(conn, h)
		if err != nil {
			return err
		}
		binding := Channel{Hello: h, Binding: hash, capacity: capacity}
		handlers.ParentControl = func(ctx context.Context, f ardp.Frame, witness, remaining uint64) error {
			if err := errors.Join(ctx.Err(), check()); err != nil {
				return err
			}
			next, err := r.config.Refill(ctx, binding, grant, remaining, f.Body[1:])
			// Even a post-spend failure must retain a returned Grant until join.
			additions = append(additions, next)
			if err = errors.Join(err, ctx.Err(), check()); err != nil {
				return err
			}
			if next.Allowance().Deadline() != h.Deadline {
				return errors.New("route refill horizon differs")
			}
			if err := s.AcceptParent(ctx, witness, next.Allowance().Bytes()); err != nil {
				return err
			}
			return errors.Join(ctx.Err(), check())
		}
	}
	s = framing.Prepare(ctx, conn, h.Deadline, grant.Allowance().Bytes()-role.AdmissionWireBytes, check, false, r.queues, handlers)
	s.Start()
	<-s.Done()
	return nil
}

func decodeOpen(body []byte) (ardp.Open, error) {
	o, err := ardp.DecodeOpen(body)
	if err != nil {
		return ardp.Open{}, err
	}
	if !time.Now().Before(o.Deadline) {
		return ardp.Open{}, errors.New("route OPEN facts invalid")
	}
	return o, nil
}

func (r *Receiver) forward(ctx context.Context, source *framing.Lane, parent ardp.Hello, body []byte, restriction ardp.ChildRestriction) (result error) {
	local, err := r.config.Authority.Hello(parent, false)
	if err != nil {
		return err
	}
	opened, err := decodeOpen(body)
	if err != nil {
		return err
	}
	forwarding := local.Subrole == 1 && opened.Purpose == 7
	registration := local.RoleDomain == 4 && local.Subrole == 2 && opened.Purpose == 4
	join := (local.RoleDomain == 1 || local.RoleDomain == 3) && local.Subrole == 2 && opened.Purpose == 6
	issuance := local.RoleDomain == 1 && local.Subrole == 2 && opened.Purpose == uint8(ardp.PurposeIssuer)
	resolution := local.RoleDomain == 1 && local.Subrole == 2 && opened.Purpose == uint8(ardp.PurposeReachability)
	submission := local.RoleDomain == 1 && local.Subrole == 2 && opened.Purpose == uint8(ardp.PurposeSubmission)
	if restriction == ardp.IssuerBootstrapChild && (local.RoleDomain != 1 || (!forwarding && !issuance)) {
		return errors.New("restricted forwarding destination unavailable")
	}
	if (!forwarding && !registration && !join && !issuance && !resolution && !submission) || opened.Deadline.After(parent.Deadline) {
		return errors.New("route prefix next-hop unavailable")
	}
	if err := source.Bound(opened.Deadline); err != nil {
		return err
	}
	v, err := r.config.Authority.Current()
	if err != nil {
		return err
	}
	next, err := v.Member(opened.RecipientNodeID, time.Now())
	expectedSubrole := uint8(2)
	expectedDomain := local.RoleDomain
	if registration {
		expectedSubrole = 3
	}
	if submission {
		expectedSubrole, expectedDomain = 3, 4
	}
	if join {
		expectedSubrole, expectedDomain = 4, 2
	}
	if issuance {
		expectedSubrole, expectedDomain = 6, 2
		if next.NodeID != v.Profile().IssuerNodeID {
			return errors.New("route exact current issuer unavailable")
		}
	}
	if resolution {
		expectedSubrole, expectedDomain = 5, 2
	}
	if err != nil || next.Subrole != expectedSubrole || next.RoleDomain != expectedDomain || next.DutyGeneration != opened.RecipientDutyGeneration || role.Conflicting(local, next) {
		return errors.Join(errors.New("route next Interior unavailable"), err)
	}
	duty, err := v.RetainDuty(next.NodeID, time.Now())
	if err != nil {
		return err
	}
	a := role.Authority{Current: r.config.Authority.Current, Duty: duty, Profile: v.Profile().ProfileBinding}
	if _, err := a.Member(); err != nil {
		return err
	}
	pooled, err := r.pool.borrow(ctx, a, func() error { _, err := a.Member(); return err }, func(ctx context.Context) (*framing.Session, func(), error) { return r.openNode(ctx, a) })
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, pooled.release()) }()
	downstream := pooled.session
	forwarded, err := ardp.EncodeNodeOpen(ardp.NodeOpen{Recipient: opened, Restriction: restriction})
	if err != nil {
		return err
	}
	target, err := downstream.Open(ctx, ctx, forwarded)
	if err != nil {
		return err
	}
	defer target.Finish()
	// Cancellation retires only this child, while healthy sibling borrowers
	// retain the same authenticated Carrier and their physical reservation.
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = target.Close(); _ = source.Close() })
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	results := make(chan error, 2)
	go func() {
		_, err := io.Copy(target, source)
		if err == nil {
			err = target.CloseWrite()
		}
		results <- err
	}()
	go func() {
		_, err := io.Copy(source, target)
		if err == nil {
			err = source.CloseWrite()
		}
		results <- err
	}()
	first := <-results
	if first != nil {
		_ = target.Close()
		_ = source.Close()
	}
	second := <-results
	return errors.Join(first, second, target.Close(), source.Close())
}

func (r *Receiver) openNode(ctx context.Context, a role.Authority) (_ *framing.Session, release func(), result error) {
	next, err := a.Member()
	if err != nil {
		return nil, nil, err
	}
	local, err := r.config.Authority.Member()
	if err != nil {
		return nil, nil, err
	}
	end := minDeadline(minDeadline(next.NotAfter(), local.NotAfter()), a.Profile.NotAfter)
	control, err := r.queues.HoldControl()
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if result != nil {
			control()
		}
	}()
	request := transport.ClosedNodeCarrierRequest{CarrierProfile: transport.CarrierProfile(next.CarrierProfile), Endpoint: next.Endpoint, Certificate: r.config.Certificate, ExpectedPeerKey: next.PublicKey, Deadline: minDeadline(end, time.Now().Add(10*time.Second))}
	if err := transport.ValidateNodeRequest(ctx, request); err != nil {
		return nil, nil, err
	}
	var physical transport.Carrier
	switch request.CarrierProfile {
	case transport.ClosedCarrierTCP:
		physical, err = roletls.OpenNode(ctx, request)
	case transport.ClosedCarrierQUIC:
		physical, err = rolequic.OpenNode(ctx, request)
	default:
		err = errors.New("closed Node Carrier profile is unsupported")
	}
	if err != nil {
		return nil, nil, err
	}
	conn, ok := physical.(net.Conn)
	if !ok {
		return nil, nil, errors.Join(errors.New("route Node Carrier missing connection contract"), physical.Close())
	}
	conn = transport.Retain(conn)
	interrupted := make(chan struct{})
	sealed := false
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = conn.SetDeadline(time.Now()); _ = conn.Close() })
	defer func() {
		if !sealed && !stop() {
			<-interrupted
		}
	}()
	defer func() {
		if result != nil {
			result = errors.Join(result, conn.Close())
		}
	}()
	h, err := a.FreshHello(end, ardp.PurposeForwarding, true)
	if err != nil {
		return nil, nil, err
	}
	if err := conn.SetDeadline(minDeadline(h.Deadline, time.Now().Add(10*time.Second))); err != nil {
		return nil, nil, err
	}
	if err := framing.SendHello(conn, h); err != nil {
		return nil, nil, err
	}
	if err := framing.Accepted(conn); err != nil {
		return nil, nil, err
	}
	if _, err := a.Member(); err != nil {
		return nil, nil, err
	}
	if err := conn.SetDeadline(h.Deadline); err != nil {
		return nil, nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	// Seal setup cancellation before publishing into the shared pool.
	if !stop() {
		<-interrupted
		return nil, nil, ctx.Err()
	}
	sealed = true
	check := func() error {
		if _, err := r.config.Authority.Member(); err != nil {
			return err
		}
		_, err := a.Member()
		return err
	}
	if err := check(); err != nil {
		return nil, nil, err
	}
	return framing.New(r.ctx, conn, h.Deadline, 32<<20, check, false, r.queues, nil), control, nil
}

func (r *Receiver) closeListener() error {
	r.listenerOnce.Do(func() { r.listenerErr = r.listener.Close() })
	return r.listenerErr
}
