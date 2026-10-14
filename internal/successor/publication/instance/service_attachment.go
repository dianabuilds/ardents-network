package instance

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
)

// ServiceAttachment confines TLS key use to one original admitted JOIN. The
// caller still owns accepting Publication, Execution and Connection checks.
// Closing it interrupts and joins TLS users before returning the JOIN borrow.
type ServiceAttachment struct {
	lifetime                              *PublicationLifetime
	lease                                 *ServiceLease
	self                                  *ServiceAttachment
	opening                               *Opening
	physical                              *join.Joined
	connection                            *tls.Conn
	transport                             *serviceTLSBorrow
	ctx                                   context.Context
	cancel                                context.CancelFunc
	stopLifetime                          func() bool
	lifetimeCallback                      chan struct{}
	watcherDone                           chan struct{}
	identity                              wire.LogicalIdentity
	generation                            uint64
	logical, attachment                   [32]byte
	setupDeadline                         time.Time
	setupContext                          context.Context
	evidence                              *wire.TLSBinding
	closed, handshakeStarted, proofIssued bool // Instance root mutex owns these.
	initialComplete                       bool
	responseStarted                       bool
	replacementWritten                    bool
	initialProof                          wire.Proof
	users                                 sync.WaitGroup // Admission Add and Close seal share that same mutex.
	once                                  sync.Once
	closeErr                              error
	cleanupErr                            error
	done                                  chan struct{}
}

// BeginService requires the original private capsule opening and consumes no
// new Instance/Grant. It returns separately owned logical and first physical
// borrows. The logical bound is original Work Safety, independent of setup
// expiry and physical retirement. Only a genuine JOIN supplies transport;
// the caller must close the logical lease even if authentication later fails.
func (lifetime *PublicationLifetime) BeginService(operation *executionruntime.Operation, opening *Opening, physical *join.Joined) (_ *ServiceLease, _ *ServiceAttachment, result error) {
	if lifetime == nil || lifetime.binding == nil || operation == nil || opening == nil || opening.recipient == nil || physical == nil {
		return nil, nil, ErrUnavailable
	}
	if err := operation.CheckPublisher(); err != nil {
		return nil, nil, err
	}
	ctx := operation.Context()
	if lifetime.caller != ctx {
		return nil, nil, ErrUnavailable
	}
	request, digest, err := opening.Request(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := physical.CheckResponderBinding(sha256.Sum256(request.JoinSecret[:]), request.HandshakeContext, time.Unix(request.WorkSafetyNotAfter, 0), request.Deadline); err != nil {
		return nil, nil, err
	}
	if end, exists := ctx.Deadline(); exists && time.Unix(request.WorkSafetyMaximum, 0).After(end) {
		return nil, nil, ErrUnavailable
	}
	// CheckResponderBinding has just matched this exact Work Safety end and
	// setup deadline and freshly observed the original JOIN. No effect occurs
	// between that check and Instance admission below.
	if err := lifetime.binding.observe(ctx); err != nil {
		return nil, nil, err
	}
	root := lifetime.binding.root
	root.mu.Lock()
	if lifetime.closed || lifetime.ctx.Err() != nil || lifetime.binding.lifetime != lifetime || opening.recipient.binding != lifetime.binding {
		root.mu.Unlock()
		return nil, nil, ErrUnavailable
	}
	binding := lifetime.binding
	if err := binding.checkLocal(ctx); err != nil {
		root.mu.Unlock()
		return nil, nil, err
	}
	value := binding.credential.Delegation()
	if !binding.consumed || len(binding.private) != ed25519.PrivateKeySize || len(binding.record) == 0 || request.Network != value.Network || request.Target != value.Target || request.PublicationDigest != sha256.Sum256(binding.record) || request.AttachmentGeneration != 1 || request.WorkSafetyMaximum > value.NotAfter.Unix() {
		root.mu.Unlock()
		return nil, nil, ErrUnavailable
	}
	identity := wire.LogicalIdentity{Network: value.Network, Target: value.Target, Instance: value.Instance, InstanceGeneration: value.Generation,
		Publication: request.PublicationDigest, Profile: request.ProfileDigest, ConnectionNonce: request.ConnectionNonce, InitiatorBinding: request.InitiatorBinding,
		WorkSafetyNotAfter: request.WorkSafetyNotAfter, WorkSafetyMaximum: request.WorkSafetyMaximum, NoNewRecoveryAfter: request.NoNewRecoveryAfter}
	logical, attachment, err := wire.Contexts(identity, digest, 1)
	if err != nil || !time.Now().Before(time.Unix(identity.WorkSafetyNotAfter, 0)) {
		root.mu.Unlock()
		return nil, nil, errors.Join(ErrUnavailable, err)
	}
	if !opening.serviceClaim.CompareAndSwap(false, true) {
		root.mu.Unlock()
		return nil, nil, ErrUnavailable
	}
	logicalContext, cancelLogical := context.WithDeadline(ctx, time.Unix(identity.WorkSafetyNotAfter, 0))
	lease := &ServiceLease{lifetime: lifetime, operation: operation, identity: identity, logical: logical, ctx: logicalContext, cancel: cancelLogical,
		attachments: make(map[*ServiceAttachment]struct{}), lifetimeCallback: make(chan struct{})}
	lease.self = lease
	lease.stopLifetime = context.AfterFunc(lifetime.ctx, func() { defer close(lease.lifetimeCallback); cancelLogical() })
	child, cancel := context.WithCancel(logicalContext)
	service := &ServiceAttachment{lifetime: lifetime, lease: lease, opening: opening, physical: physical, ctx: child, cancel: cancel, identity: identity, logical: logical, attachment: attachment,
		generation: 1, setupDeadline: request.Deadline, setupContext: opening.ctx, lifetimeCallback: make(chan struct{}), watcherDone: make(chan struct{})}
	service.self = service
	service.stopLifetime = context.AfterFunc(lifetime.ctx, func() { defer close(service.lifetimeCallback); cancel() })
	err = service.configureTLSLocked()
	if err != nil {
		cancel()
		root.mu.Unlock()
		if !service.stopLifetime() {
			<-service.lifetimeCallback
		}
		return nil, nil, errors.Join(err, lease.Close())
	}
	lifetime.connections[lease] = struct{}{}
	lease.attachments[service] = struct{}{}
	root.mu.Unlock()
	go service.watch()
	if _, _, err := opening.Request(ctx); err != nil {
		return nil, nil, errors.Join(err, lease.Close())
	}
	if err := physical.CheckOriginal(); err != nil {
		return nil, nil, errors.Join(err, lease.Close())
	}
	if err := lease.CheckIdentity(identity, logical); err != nil {
		return nil, nil, errors.Join(err, lease.Close())
	}
	return lease, service, nil
}

func (service *ServiceAttachment) configureTLSLocked() error {
	service.done = make(chan struct{})
	certificate, err := service.certificateLocked()
	if err != nil {
		return err
	}
	service.transport = &serviceTLSBorrow{service: service}
	service.connection = tls.Server(service.transport, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519}, Certificates: []tls.Certificate{certificate}, SessionTicketsDisabled: true})
	return nil
}

func (service *ServiceAttachment) certificateLocked() (tls.Certificate, error) {
	if err := service.lease.operation.CheckPublisher(); err != nil {
		return tls.Certificate{}, err
	}
	value := service.lifetime.binding.credential.Delegation()
	template := &x509.Certificate{SerialNumber: new(big.Int).SetUint64(value.Generation), NotBefore: value.NotBefore, NotAfter: value.NotAfter,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, SignatureAlgorithm: x509.PureEd25519}
	public := append(ed25519.PublicKey(nil), value.Instance[:]...)
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, service.lifetime.binding.private)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := service.lease.operation.CheckPublisher(); err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: &serviceCertificateSigner{service: service}}, nil
}

// checkLocked is the local effect/commit gate after recheck outside root.mu.
func (service *ServiceAttachment) checkLocked() error {
	lifetime := service.lifetime
	if service.self != service || service.lease == nil {
		return ErrUnavailable
	}
	if err := service.lease.checkCommitLocked(); err != nil {
		return err
	}
	if service.closed || service.ctx.Err() != nil || lifetime.closed || lifetime.ctx.Err() != nil || lifetime.binding.lifetime != lifetime {
		return ErrUnavailable
	}
	if !service.initialComplete && (service.setupContext == nil || service.setupContext.Err() != nil || !time.Now().Before(service.setupDeadline)) {
		return ErrUnavailable
	}
	if !service.initialComplete {
		if service.opening == nil || service.opening.recipient == nil {
			return ErrUnavailable
		}
		if err := service.opening.recipient.checkLocal(service.ctx); err != nil {
			return err
		}
	}
	if _, retained := service.lease.attachments[service]; !retained {
		return ErrUnavailable
	}
	return lifetime.binding.checkLocal(service.ctx)
}

func (service *ServiceAttachment) enter() error {
	if service == nil || service.lifetime == nil || service.connection == nil || service.physical == nil {
		return ErrUnavailable
	}
	if err := service.recheck(); err != nil {
		return err
	}
	root := service.lifetime.binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := service.checkLocked(); err != nil {
		return err
	}
	service.users.Add(1)
	return nil
}

func (service *ServiceAttachment) recheck() error {
	if service == nil || service.self != service || service.lifetime == nil || service.lifetime.binding == nil || service.physical == nil || service.lease == nil {
		return ErrUnavailable
	}
	if err := service.physical.CheckOriginal(); err != nil {
		return err
	}
	if err := service.lease.CheckIdentity(service.identity, service.logical); err != nil {
		return err
	}
	root := service.lifetime.binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	return service.checkLocked()
}

// Handshake authenticates only this TLS Attachment, not the logical stream.
func (service *ServiceAttachment) Handshake(ctx context.Context) error {
	if ctx == nil {
		return ErrUnavailable
	}
	if err := service.enter(); err != nil {
		return err
	}
	defer service.users.Done()
	root := service.lifetime.binding.root
	root.mu.Lock()
	if service.handshakeStarted {
		root.mu.Unlock()
		return ErrUnavailable
	}
	service.handshakeStarted = true
	root.mu.Unlock()
	if err := service.physical.SetDeadline(service.setupDeadline); err != nil {
		return err
	}
	caller, cancel := context.WithCancel(service.ctx)
	callbackDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(callbackDone); cancel() })
	err := service.connection.HandshakeContext(caller)
	if !stop() {
		<-callbackDone
	}
	err = errors.Join(err, caller.Err(), ctx.Err())
	cancel()
	if err != nil {
		return err
	}
	evidence, err := wire.BindTLS(service.connection, service.attachment)
	if err != nil {
		return err
	}
	if err := service.recheck(); err != nil {
		return err
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := service.checkLocked(); err != nil {
		return err
	}
	service.evidence = evidence
	return nil
}

// Evidence exposes sealed completed TLS provenance and Connection secrets,
// while the certificate signer/config and original private key stay confined.
func (service *ServiceAttachment) Evidence() (*wire.TLSBinding, error) {
	if err := service.recheck(); err != nil {
		return nil, err
	}
	root := service.lifetime.binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if service.evidence == nil {
		return nil, ErrUnavailable
	}
	return service.evidence, service.checkLocked()
}

// CheckIdentity matches this original private opening's sealed tuple. It
// supplies neither accepting Publication nor local worker authority.
func (service *ServiceAttachment) CheckIdentity(identity wire.LogicalIdentity, logical, attachment [32]byte) error {
	if err := service.recheck(); err != nil {
		return err
	}
	return service.MatchIdentity(identity, logical, attachment)
}

// MatchIdentity compares the original sealed tuple and local lifetime only.
// It supplies no accepting observation. Read/Write independently perform full
// original-owner observations before and after I/O; other consumers must use
// CheckIdentity before an accepting effect.
func (service *ServiceAttachment) MatchIdentity(identity wire.LogicalIdentity, logical, attachment [32]byte) error {
	if service == nil || service.self != service || service.lifetime == nil || service.lifetime.binding == nil || service.lifetime.binding.root == nil {
		return ErrUnavailable
	}
	root := service.lifetime.binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if service.identity != identity || service.logical != logical || service.attachment != attachment {
		return ErrUnavailable
	}
	return service.checkLocked()
}

// CheckStreamCommit is the local initial-stream gate after CheckIdentity and
// under the original JOIN handoff barrier. No native observation, signature or
// transport effect occurs here; it cannot replace authentication/currentness.
func (service *ServiceAttachment) CheckStreamCommit(identity wire.LogicalIdentity, logical, attachment [32]byte) error {
	if service == nil || service.self != service || service.lifetime == nil || service.lifetime.binding == nil ||
		service.lifetime.binding.root == nil || service.lease == nil || service.lease.lifetime != service.lifetime {
		return ErrUnavailable
	}
	root := service.lifetime.binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if service.closed || !service.initialComplete || service.generation != 1 || service.evidence == nil || service.ctx == nil || service.ctx.Err() != nil ||
		service.identity != identity || service.logical != logical || service.attachment != attachment || service.lease.identity != identity || service.lease.logical != logical {
		return ErrUnavailable
	}
	if _, retained := service.lease.attachments[service]; !retained {
		return ErrUnavailable
	}
	return service.lease.checkCommitLocked()
}

// CommitStream lends only the original admitted JOIN's local handoff barrier.
// The consumer rechecks this Instance through CheckStreamCommit in its bounded
// callback; no native observation or physical effect may run in that callback.
func (service *ServiceAttachment) CommitStream(ctx context.Context, handoff func(func() error) error) error {
	if service == nil || service.self != service || service.physical == nil {
		return ErrUnavailable
	}
	return service.physical.CommitOriginal(ctx, handoff)
}

func (service *ServiceAttachment) Context() context.Context {
	if service == nil {
		return nil
	}
	return service.ctx
}

func (service *ServiceAttachment) Read(body []byte) (int, error) {
	if err := service.enter(); err != nil {
		return 0, err
	}
	defer service.users.Done()
	root := service.lifetime.binding.root
	root.mu.Lock()
	ready := service.evidence != nil
	root.mu.Unlock()
	if !ready {
		return 0, ErrUnavailable
	}
	n, err := service.connection.Read(body)
	return n, errors.Join(err, service.recheck())
}

func (service *ServiceAttachment) Write(body []byte) (int, error) {
	if err := service.enter(); err != nil {
		return 0, err
	}
	defer service.users.Done()
	root := service.lifetime.binding.root
	root.mu.Lock()
	ready := service.initialComplete
	root.mu.Unlock()
	if !ready {
		return 0, ErrUnavailable
	}
	n, err := service.connection.Write(body)
	return n, errors.Join(err, service.recheck())
}

func (service *ServiceAttachment) watch() {
	defer close(service.watcherDone)
	<-service.ctx.Done()
	// Deadline interrupts all TLS users; physical release waits in Close.
	_ = service.physical.InterruptIO()
}

func (service *ServiceAttachment) Close() error {
	return service.close(false)
}

// CloseFailed retains and joins an unsuccessful original Attachment without
// emitting a clean Route terminal that could retire the peer's control tail.
func (service *ServiceAttachment) CloseFailed() error {
	return service.close(true)
}

func (service *ServiceAttachment) close(failed bool) error {
	if service == nil || service.lifetime == nil {
		return nil
	}
	if service.self != service {
		return ErrUnavailable
	}
	service.once.Do(func() {
		var disposition error
		if failed && service.physical != nil {
			disposition = service.physical.MarkFailedRetirement()
		}
		root := service.lifetime.binding.root
		root.mu.Lock()
		service.closed = true
		service.cancel()
		root.mu.Unlock()
		<-service.watcherDone
		service.users.Wait()
		if service.transport == nil || service.physical == nil {
			service.closeErr = ErrUnavailable
			service.cleanupErr = ErrUnavailable
		} else {
			interruption := service.transport.Close()
			service.closeErr = errors.Join(disposition, interruption, service.physical.Close())
			cleanup, joined := service.physical.CleanupResult()
			if !joined {
				cleanup = errors.Join(cleanup, ErrUnavailable)
			}
			service.cleanupErr = errors.Join(interruption, cleanup)
		}
		if !service.stopLifetime() {
			<-service.lifetimeCallback
		}
		root.mu.Lock()
		if service.evidence != nil {
			service.evidence.Erase()
		}
		if service.lease != nil {
			service.lease.physicalFailure = errors.Join(service.lease.physicalFailure, service.cleanupErr)
			delete(service.lease.attachments, service)
		}
		root.mu.Unlock()
		if service.done != nil {
			close(service.done)
		}
	})
	return service.closeErr
}

// CleanupResult exposes only the original physically joined retirement result.
// Close retains EOF/refusal separately, and neither result is Service authority.
func (service *ServiceAttachment) CleanupResult() (error, bool) {
	if service == nil || service.self != service || service.physical == nil || service.done == nil {
		return nil, false
	}
	select {
	case <-service.done:
		return service.cleanupErr, true
	default:
		return nil, false
	}
}
