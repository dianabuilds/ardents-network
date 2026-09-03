package route

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// StateView is the exact authenticated State projection needed for one User
// route. State remains the owner of freshness, role selection, and candidate
// validity; Route receives no candidate list or State root.
type StateView interface {
	Epoch(time.Time, time.Time) (state.ResolutionEpoch, bool)
	Candidate([32]byte, time.Time, time.Time) (state.ResolutionCandidate, bool)
	Gateway(time.Time, time.Time) (state.DestinationResolutionGateway, bool)
	CredentialIssuer(time.Time, time.Time) (state.TransitIssuer, bool)
}

// UserEntry is Route's narrow port to the Endpoint-owned durable Entry
// contact. Entry retains invite validation, retry, and cleanup ownership.
type UserEntry interface {
	EntryAcquirer
	Contact() (entry.Candidate, error)
}

// ResourceAdmission reserves Endpoint-owned capacity for one whole User
// route attempt. Route calls the returned release exactly once on every
// failure or when the resulting Attachment closes.
type ResourceAdmission func(context.Context) (release func() error, err error)

// TransitPeer is one exact State-selected adjacent hop. It is an opaque
// operation carrier for the local credential adapter, never a Route plan.
type TransitPeer struct {
	NodeID, PublicKey, Family [32]byte
	Endpoint                  string
}

// CredentialExchange carries one OHTTP envelope through Route's exact
// Entry-to-Initiator Credential Relay. It has no destination or retry input.
type CredentialExchange func(context.Context, []byte) ([]byte, error)

// CredentialRequest contains the exact State-bound tuple whose durable,
// at-most-once grant lifecycle remains Endpoint-owned. Exchange is Route's
// carrier adapter, preventing the credential owner from selecting a peer.
type CredentialRequest struct {
	Epoch        state.ResolutionEpoch
	Issuer       state.TransitIssuer
	Initiator    TransitPeer
	Transit      TransitPeer
	TransitRole  byte
	AttachmentID [32]byte
	At           time.Time
	NotAfter     time.Time
	Exchange     CredentialExchange
}

// Credential is one already acquired membership Grant plus its local,
// key-bound TLS identity. Finish must durably record whether the credential
// reached a receiving transit Node; Route calls it exactly once.
type Credential struct {
	Authorization     []byte
	ClientCertificate tls.Certificate
	Finish            func(presented bool) error
}

// CredentialAcquirer is Endpoint's local adapter to its durable membership
// Grant journal. It has production and behavior-test adapters, while Route
// retains all transport and peer composition.
type CredentialAcquirer func(context.Context, CredentialRequest) (Credential, error)

// Config binds the one endpoint-local User Route owner to its current State,
// Entry, durable credential adapter, capacity owner, and clock. No caller can
// inject a Target Link, Gateway URL, Route plan, or alternate peer.
type Config struct {
	NetworkID   [32]byte
	Current     func() (StateView, error)
	Entry       UserEntry
	Credentials CredentialAcquirer
	Admit       ResourceAdmission
	Clock       func() time.Time
}

// Intent is one authenticated Target selected by Endpoint's Service Link
// binding. The caller has no deadline, peer, descriptor, grant, or fallback.
type Intent struct{ Target [32]byte }

// Evidence is the immutable, verified fact Service Connection needs after a
// Route opens. It deliberately exposes no State selection or credential data.
type Evidence struct {
	AuthenticatedTarget [32]byte
	AuthorityPublic     [32]byte
	Publication         []byte
	Generation          uint64
	AttachmentID        [32]byte
}

// Route owns the complete volatile User-route composition. It cancels and
// joins all pending work before closing active Attachments.
type Route struct {
	mu              sync.Mutex
	config          Config
	done            context.CancelFunc
	doneCtx         context.Context
	closed          bool
	active          map[*Attachment]struct{}
	terminalFailure error
	flight          int
	idle            *sync.Cond
}

// Open creates the User Route owner. It performs no State read, admission,
// Entry acquisition, or network operation until Attach.
func Open(input Config) (*Route, error) {
	if input.NetworkID == [32]byte{} || input.Current == nil || input.Entry == nil || input.Credentials == nil || input.Admit == nil {
		return nil, errors.New("user Route configuration is incomplete")
	}
	if input.Clock == nil {
		input.Clock = time.Now
	}
	doneCtx, done := context.WithCancel(context.Background())
	result := &Route{config: input, done: done, doneCtx: doneCtx, active: make(map[*Attachment]struct{})}
	result.idle = sync.NewCond(&result.mu)
	return result, nil
}

// Attach opens exactly one State-selected private reachability and C-2 route
// to Intent.Target. It never ranks candidates, retries an alternate, or
// falls back to a direct Service connection.
func (route *Route) Attach(ctx context.Context, intent Intent) (*Attachment, error) {
	if route == nil || ctx == nil || intent.Target == [32]byte{} {
		return nil, errors.New("user Route attachment intent is invalid")
	}
	if err := route.beginAttach(); err != nil {
		return nil, err
	}
	defer route.endAttach()
	attempt, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(route.doneCtx, cancel)
	defer func() { stop(); cancel() }()
	release, err := route.config.Admit(attempt)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, errors.New("user Route resource admission has no release")
	}
	attachment, err := route.openUserAttachment(attempt, intent, release)
	if err != nil {
		return nil, err
	}
	route.mu.Lock()
	attachment.bindCompletion(func(result error) { route.complete(attachment, result) })
	if route.closed {
		route.mu.Unlock()
		return nil, errors.Join(errors.New("user Route is closed"), attachment.Close())
	}
	route.active[attachment] = struct{}{}
	route.mu.Unlock()
	return attachment, nil
}

// Close cancels and joins all pending attempts, then closes every active
// attachment. A nil result proves no Route-owned admission or carrier remains.
func (route *Route) Close() error {
	if route == nil {
		return errors.New("user Route is unavailable")
	}
	route.mu.Lock()
	if route.closed {
		route.mu.Unlock()
		return errors.New("user Route is closed")
	}
	route.closed = true
	route.done()
	for route.flight != 0 {
		route.idle.Wait()
	}
	attachments := make([]*Attachment, 0, len(route.active))
	for attachment := range route.active {
		attachments = append(attachments, attachment)
	}
	route.mu.Unlock()
	var result error
	for _, attachment := range attachments {
		result = errors.Join(result, attachment.Close())
	}
	route.mu.Lock()
	remembered := route.terminalFailure
	route.mu.Unlock()
	if remembered == nil || errors.Is(result, remembered) {
		return result
	}
	return errors.Join(result, remembered)
}

func (route *Route) beginAttach() error {
	route.mu.Lock()
	defer route.mu.Unlock()
	if route.closed {
		return errors.New("user Route is closed")
	}
	route.flight++
	return nil
}

func (route *Route) endAttach() {
	route.mu.Lock()
	route.flight--
	if route.flight == 0 {
		route.idle.Broadcast()
	}
	route.mu.Unlock()
}

func (route *Route) complete(attachment *Attachment, result error) {
	route.mu.Lock()
	attachment.publish(result)
	delete(route.active, attachment)
	if route.terminalFailure == nil && result != nil {
		route.terminalFailure = result
	}
	route.mu.Unlock()
}
