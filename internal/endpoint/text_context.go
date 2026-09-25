//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/endpoint/descriptorhistory"
)

// textContextState owns platform-independent local authorization for a context.
// It is never a wire identity or evidence of installed confinement. Only the
// verified launch boundary may give a worker a Principal and Grant.
type textContextState struct {
	refreshFailure    func(string)
	withdrawalFailure func(string)
	operationFailure  func(string)
	refresh           textPublicationRefreshLifecycle
	textPublicationPairLifecycle
	introductionDispatch  textIntroductionDispatch
	introductionExchanges map[*textIntroductionExchange]struct{}
	introductionAdmission textIntroductionAdmission
	descriptorHistory     descriptorhistory.History
	withdrawal            *textSourceFlight
	registrationOpening   *textRegistrationFlight
	introduction          textIntroductionPrefixLifecycle
	responder             textResponderPrefixLifecycle
	resolution            *textResolutionFlight
	source                textSourceLifecycle
	sourceSet             *textSourceSet
	sourceOperations      chan struct{}
	issuance              *textIssuanceOperation
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

func (owner *textContext) reportTextOperationFailure(failure string) {
	owner.mu.Lock()
	report := owner.operationFailure
	owner.mu.Unlock()
	if report != nil {
		report(failure)
	}
}

// reportTextWithdrawalFailure exposes one fixed local operational category.
// It never serializes a wrapped error, peer, route, document, or authority.
func (owner *textContext) reportTextWithdrawalFailure(failure string) {
	owner.mu.Lock()
	report := owner.withdrawalFailure
	owner.mu.Unlock()
	if report != nil {
		report(failure)
	}
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
	job, err := newTextJobIdentity(owner)
	if err != nil {
		return nil, err
	}
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
	return job.currentLocked(owner, endpoint, surface, nonce)
}

// retireJob invalidates completions and interrupts invocation I/O before
// cleanup starts. The separately authorized Endpoint context survives.
func (owner *textContext) retireJob(job *textJobIdentity) {
	if job != nil {
		job.retire()
	}
}

// finishJobCleanup publishes one immutable cleanup outcome. A later callback
// cannot erase failure or release a replacement's reservation. This internal
// completion is not installed confinement evidence.
func (owner *textContext) finishJobCleanup(job *textJobIdentity, cleanupErr error) error {
	if job == nil || job.owner != owner {
		return errors.New("text context is unavailable")
	}
	return job.finishCleanup(cleanupErr)
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
	retirement := owner.stopTextContextChildrenLocked()
	owner.mu.Unlock()
	owner.closeErr = errors.Join(owner.closeErr, retirement.join())
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
