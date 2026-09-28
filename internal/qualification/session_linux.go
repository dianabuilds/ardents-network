//go:build linux

package qualification

import (
	"context"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// Session is the bounded, authorized participant operation set that the
// scenario owner consumes during one measured run. Endpoint implements it;
// every operation rechecks its own authority under the Context lock. Private
// Endpoint state — Context, Job identity, permission files, Source handles,
// worker internals — never crosses this boundary: only plain values, opaque
// handles, and the retained Run.
type Session interface {
	PublisherWorker

	// Artifact returns the verified-worker Artifact for observation.
	Artifact() *Artifact
	// Cgroup returns the worker cgroup identity for measurement registration.
	Cgroup() string
	// BeginOperation starts an authorized broker operation on the scenario
	// surface and returns the bounded context plus its completion function.
	BeginOperation(ctx context.Context) (context.Context, func(), error)
	// ProvisionPermission performs the scenario permission handover,
	// including the participant approval report callback.
	ProvisionPermission(ctx context.Context) error
	// ResolveIntroduction resolves the Target Link to a verified Descriptor.
	ResolveIntroduction(ctx context.Context, destination targetlink.Link) (reachability.Verified, error)
	// PrepareIntroduction reserves the exact Introduction state for one
	// resolved setup and returns its opaque handle.
	PrepareIntroduction(ctx context.Context, destination targetlink.Link, bounds [3]int64, verified reachability.Verified) (Preparation, error)
	// OpenJoinedService completes setup admission and opens the joined
	// Service stream; setupComplete runs exactly once when the setup phase
	// ends.
	OpenJoinedService(ctx context.Context, preparation Preparation, setupComplete func()) (connection.Stream, error)
	// ReserveRecipients reads the current reserve recipients for one setup
	// attempt. The error covers the unavailable Source prefix.
	ReserveRecipients() (ReserveRecipients, error)
	// EnsureTokenReserve authorizes the token stock behind one receiver for
	// the required class minimum.
	EnsureTokenReserve(ctx context.Context, receiver [32]byte, class uint8, minimum int) error
	// StartPublication starts the authorized Publisher publication and
	// returns its handle.
	StartPublication(ctx context.Context) (Publication, error)
}

// PublisherWorker is the subset of Session used while the fixed workload
// runs: replenishment and stream service. Both Reader and Publisher paths
// use it.
type PublisherWorker interface {
	// Run returns the retained per-invocation Run, or nil when the job has
	// no qualification run bound.
	Run() *Run
	// Now returns the participant clock reading used for setup deadlines.
	Now() time.Time
	// WorkerAttachment returns the attached worker transport for the
	// workload Attachment bridge.
	WorkerAttachment() io.ReadWriteCloser
	// RefillSnapshot reads the live-job, busy-issuance, and joined-stream
	// state in one atomic Context inspection.
	RefillSnapshot() Refill
	// EnsureIssuerReserve authorizes the issuer token stock for the minimum
	// duty-1 reserve.
	EnsureIssuerReserve(ctx context.Context, minimum int) error
	// ReplenishPrefixes replenishes the current Source, Introduction, and
	// responder prefixes through the presented tokens.
	ReplenishPrefixes(ctx context.Context, present client.ClosedTokenPresenter) error
	// PresentRefill presents one qualified refill for the bound job under
	// the current authority checks.
	PresentRefill(ctx context.Context, hello ardp.Hello, class uint8) ([]byte, error)
	// Close retires the worker with joined completion.
	Close() error
}

// Preparation is an opaque Endpoint-owned Introduction preparation handle.
// The marker method keeps foreign types from satisfying it.
type Preparation interface {
	QualificationPreparation()
}

// Publication is an Endpoint-owned publication flight observed by the
// Publisher scenario branch.
type Publication interface {
	// Link returns the published Target Link once the flight is registered.
	Link() targetlink.Link
	// Done is closed when the publication flight ends.
	Done() <-chan struct{}
	// Err returns the flight failure; valid after Done closes.
	Err() error
	// Close cancels the publication and joins its completion.
	Close() error
}

// ReserveRecipients is one atomic read of the JOIN and submission reserve
// recipients for a setup attempt. JoinErr and SubmissionErr carry the
// per-recipient failures at their original check positions.
type ReserveRecipients struct {
	Join          [32]byte
	JoinErr       error
	Submission    [32]byte
	SubmissionErr error
}

// Refill is one atomic refill inspection under the Context lock.
type Refill struct {
	// Live reports the qualification job as the current live text service
	// job on the scenario surface.
	Live bool
	// Busy reports an issuance operation in progress; refill is skipped.
	Busy bool
	// Joins are the joined streams retained by the Run at inspection time.
	Joins []*client.ClosedJoinedStream
}
