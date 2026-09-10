//go:build linux

package endpoint

import (
	"context"
	"crypto/rand"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// textContextState owns platform-independent local authorization for a context.
// It is never a wire identity or evidence of installed confinement. Only the
// verified launch boundary may give a worker a Principal and Grant.
type textContextState struct {
	publicationDraining   bool
	refresh               *textPublicationRefresh
	previousRegistration  *textIntroductionRegistration
	previousUntil         time.Time
	registrationChanged   chan struct{}
	introductionExchanges map[*textIntroductionExchange]struct{}
	introductionReplays   map[[32]byte]time.Time
	introductionOpenings  [4]time.Time
	descriptorFloors      map[[32]byte]textDescriptorFloor
	withdrawal            *textSourceFlight
	registration          *textIntroductionRegistration
	registrationOpening   *textRegistrationFlight
	introduction          textPublisherPrefix
	responder             textPublisherPrefix
	resolution            *textResolutionFlight
	prefix                *route.ClosedSourcePrefix
	prefixCancel          context.CancelFunc
	prefixOpening         *textSourceFlight
	sourceSet             *textSourceSet
	issuance              *textIssuanceFlight
	mu                    sync.Mutex
	endpoint              *endpoint
	lease                 *broker.ActiveSession
	principal             [32]byte
	surface               broker.Surface
	job                   *textJobIdentity
	lastJob               *textJobIdentity
	verifiedJob           *textJobIdentity
	permission            *textPermission
	closed                bool
	done                  chan struct{}
	closeErr              error
}

// textJobIdentity reserves one invocation through joined cleanup, including
// cancellation during launch. Possession is not a qualified-launch receipt.
type textJobIdentity struct {
	owner       *textContext
	nonce       [32]byte
	context     context.Context
	cancel      context.CancelFunc
	done        chan struct{}
	retired     bool
	bound       bool
	finished    bool
	cleanupErr  error
	workerGrant *broker.Broker
}

// beginTextContext consumes existing local authority before any worker launch
// or destination-dependent effect. Caller-supplied identifiers cannot create
// a context or change a Connection authorization into a Publisher role.
func (endpoint *endpoint) beginTextContext(ctx context.Context, capability, principal [32]byte, surface broker.Surface) (*textContext, error) {
	if endpoint == nil || endpoint.admission == nil || ctx == nil ||
		(surface != broker.Connection && surface != broker.Administration) {
		return nil, errors.New("text context authorization is unavailable")
	}
	lease, _, err := endpoint.admission.Activate(ctx, capability, principal, surface)
	if err != nil {
		return nil, errors.New("text context authorization is unavailable")
	}
	owner := &textContext{textContextState: textContextState{endpoint: endpoint, lease: lease, principal: principal, surface: surface, done: make(chan struct{})}}
	if err := endpoint.retainTextContext(owner); err != nil {
		lease.Release()
		return nil, err
	}
	go owner.closeAfterAuthorization()
	return owner, nil
}

// beginJob requires an explicit owner action after the previous invocation
// has retired. Its caller must finishJobCleanup on every exit, even if launch
// fails before an attachment exists. Close joins that reservation as well.
func (owner *textContext) beginJob(endpoint *endpoint, surface broker.Surface) (*textJobIdentity, error) {
	if owner == nil {
		return nil, errors.New("text context is unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.liveLocked(endpoint, surface) || owner.job != nil {
		return nil, errors.New("text context is unavailable or already has a worker")
	}
	job := &textJobIdentity{owner: owner, done: make(chan struct{})}
	if _, err := rand.Read(job.nonce[:]); err != nil || job.nonce == [32]byte{} {
		return nil, errors.New("text worker identity is unavailable")
	}
	job.context, job.cancel = context.WithCancel(owner.lease.Context())
	owner.job, owner.lastJob = job, job
	return job, nil
}

// currentJob is checked at every asynchronous completion. A copied nonce,
// old job pointer, foreign Endpoint, or opposite local role cannot attach.
func (owner *textContext) currentJob(endpoint *endpoint, surface broker.Surface, job *textJobIdentity, nonce [32]byte) bool {
	if owner == nil {
		return false
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.liveLocked(endpoint, surface) && job != nil && !job.retired && owner.job == job &&
		job.owner == owner && nonce != [32]byte{} && job.nonce == nonce
}

// retireJob invalidates completions and interrupts invocation I/O before
// cleanup starts. The separately authorized Endpoint context survives.
func (owner *textContext) retireJob(job *textJobIdentity) {
	if owner == nil {
		return
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if job != nil && owner.job == job && job.owner == owner {
		owner.retireJobLocked(job)
	}
}

func (owner *textContext) retireJobLocked(job *textJobIdentity) {
	clear(job.nonce[:])
	job.retired = true
	job.cancel()
	if job.workerGrant != nil {
		job.workerGrant.Close()
	}
}

// finishJobCleanup publishes one immutable cleanup outcome. A later callback
// cannot erase failure or release a replacement's reservation. This internal
// completion is not installed confinement evidence.
func (owner *textContext) finishJobCleanup(job *textJobIdentity, cleanupErr error) error {
	if owner == nil {
		return errors.New("text context is unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if job == nil || job.owner != owner {
		return errors.New("text worker cleanup does not match the retired job")
	}
	if job.finished {
		return job.cleanupErr
	}
	if owner.job != job || !job.retired {
		return errors.New("text worker cleanup does not match the retired job")
	}
	job.finished, job.cleanupErr = true, cleanupErr
	if cleanupErr != nil {
		owner.closed = true
		owner.endpoint.failTextContexts(cleanupErr)
	} else {
		owner.job = nil
	}
	close(job.done)
	return cleanupErr
}

func (owner *textContext) liveLocked(endpoint *endpoint, surface broker.Surface) bool {
	return !owner.closed && owner.endpoint != nil && owner.endpoint == endpoint && owner.endpoint.textAvailable() && owner.surface == surface &&
		owner.lease != nil && owner.lease.Context().Err() == nil
}

// closeAfterAuthorization is the sole terminal owner. Revoke, parent cancel,
// explicit Close and Endpoint loss all join the same pending launch/cleanup.
func (owner *textContext) closeAfterAuthorization() {
	<-owner.lease.Context().Done()
	owner.lease.Release()
	owner.mu.Lock()
	owner.closed = true
	refresh := owner.refresh
	if refresh != nil {
		refresh.cancel()
	}
	previous := owner.previousRegistration
	if previous != nil {
		previous.cancel()
	}
	owner.previousRegistration = nil
	owner.signalTextRegistrationsLocked()
	exchanges := make([]*textIntroductionExchange, 0, len(owner.introductionExchanges))
	for flight := range owner.introductionExchanges {
		flight.cancel()
		exchanges = append(exchanges, flight)
	}
	withdrawal := owner.withdrawal
	if withdrawal != nil {
		withdrawal.cancel()
	}
	registered, registrationOpening := owner.registration, owner.registrationOpening
	if registered != nil {
		registered.cancel()
	}
	if registrationOpening != nil {
		registrationOpening.cancel()
	}
	owner.registration = nil
	clear(owner.introductionReplays)
	owner.introductionReplays = nil
	owner.introductionOpenings = [4]time.Time{}
	clear(owner.descriptorFloors)
	owner.descriptorFloors = nil
	owner.sourceSet, owner.introduction.set, owner.responder.set = nil, nil, nil
	introduction, introductionOpening := owner.introduction.prefix, owner.introduction.opening
	if owner.introduction.cancel != nil {
		owner.introduction.cancel()
	}
	if introductionOpening != nil {
		introductionOpening.cancel()
	}
	owner.introduction.prefix = nil
	responder, responderOpening := owner.responder.prefix, owner.responder.opening
	if owner.responder.cancel != nil {
		owner.responder.cancel()
	}
	if responderOpening != nil {
		responderOpening.cancel()
	}
	owner.responder.prefix = nil
	prefix, opening := owner.prefix, owner.prefixOpening
	if owner.prefixCancel != nil {
		owner.prefixCancel()
	}
	if opening != nil {
		opening.cancel()
	}
	owner.prefix = nil
	owner.clearTextPermissionLocked()
	issuance := owner.issuance
	if issuance != nil {
		issuance.cancel()
	}
	resolution := owner.resolution
	if resolution != nil {
		resolution.cancel()
	}
	job := owner.job
	if job != nil {
		owner.retireJobLocked(job)
	}
	owner.mu.Unlock()
	if opening != nil {
		<-opening.done
	}
	if introductionOpening != nil {
		<-introductionOpening.done
	}
	if responderOpening != nil {
		<-responderOpening.done
	}
	if registrationOpening != nil {
		<-registrationOpening.done
	}
	if refresh != nil {
		<-refresh.done
	}
	var prefixErr error
	if previous != nil {
		prefixErr = previous.close()
	}
	if registered != nil {
		prefixErr = errors.Join(prefixErr, registered.close())
	}
	if introduction != nil {
		prefixErr = errors.Join(prefixErr, introduction.Close())
	}
	if responder != nil {
		prefixErr = errors.Join(prefixErr, responder.Close())
	}
	if prefix != nil {
		prefixErr = errors.Join(prefixErr, prefix.Close())
	}
	if issuance != nil {
		<-issuance.done
	}
	if resolution != nil {
		<-resolution.done
	}
	if withdrawal != nil {
		<-withdrawal.done
	}
	for _, flight := range exchanges {
		<-flight.done
	}
	owner.closeErr = errors.Join(owner.closeErr, prefixErr)
	if job != nil {
		<-job.done
		owner.closeErr = errors.Join(owner.closeErr, job.cleanupErr)
	}
	owner.closeErr = errors.Join(owner.closeErr, owner.retireTextPublication())
	owner.endpoint.releaseTextContext(owner, owner.closeErr)
	close(owner.done)
}

// Close revokes local use immediately, then joins even an in-flight launch.
// Its caller must not own an unfinished job on the same execution stack.
// Repeated Close returns the first joined cleanup result.
func (owner *textContext) Close() error {
	if owner == nil {
		return nil
	}
	owner.lease.Release()
	<-owner.done
	return owner.closeErr
}
