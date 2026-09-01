package route

import (
	"context"
	"crypto/rand"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// View is Route's opaque read-only authenticated Network State seam. Current
// returns one immutable durably published projection; Route never opens a
// State root or parses an epoch representation.
type View interface {
	Current() (state.Snapshot, error)
}

// ResourceAdmission reserves caller-owned capacity for exactly one native
// attachment. The returned release must be callable after a partial Entry
// failure as well as after ordinary attachment close.
type ResourceAdmission func(context.Context) (release func() error, err error)

// Config supplies the three opaque facts Route needs to own volatile native
// attachment life cycles. It intentionally has no Node listener, capacity
// profile, Service Target, Application stream, or serialized Route plan.
type Config struct {
	View  View
	Entry EntryAcquirer
	Admit ResourceAdmission
}

// Intent is the caller's one bounded request for a fresh attachment. The
// deadline is a safety ceiling: Route additionally shortens it to current
// State, candidate, and duty validity and never accepts a caller-provided
// selection or Node identity.
type Intent struct {
	Deadline time.Time
}

// Route owns one endpoint-local native Route selection and every volatile
// attachment reservation. Service Connection decides whether to request a
// replacement; it cannot observe or direct Route's selected Node positions.
type Route struct {
	mu      sync.Mutex
	config  Config
	done    context.CancelFunc
	doneCtx context.Context
	closed  bool
	pending map[[32]byte]plan
	active  map[[32]byte]activeAttachment
	flight  int
	idle    *sync.Cond
}

type activeAttachment struct {
	plan       plan
	attachment *Attachment
}

// Open creates a native Route lifecycle. It performs no admission, State
// read, TCP dial, or Entry operation; Attach owns each bounded attempt.
func Open(input Config) (*Route, error) {
	if input.View == nil || input.Entry == nil || input.Admit == nil {
		return nil, errors.New("native Route configuration is incomplete")
	}
	doneCtx, done := context.WithCancel(context.Background())
	result := &Route{config: input, done: done, doneCtx: doneCtx, pending: make(map[[32]byte]plan), active: make(map[[32]byte]activeAttachment)}
	result.idle = sync.NewCond(&result.mu)
	return result, nil
}

// Attach selects an internally private fresh Route and opens its adjacent
// Entry attachment. Resource admission happens before Entry acquisition, and
// parallel Attach calls reserve disjoint positions until each Attachment is
// closed. Route never uses a direct Service connection as a fallback.
func (route *Route) Attach(ctx context.Context, intent Intent) (*Attachment, error) {
	if route == nil || ctx == nil || intent.Deadline.IsZero() || !time.Now().Before(intent.Deadline) {
		return nil, errors.New("native Route attachment intent is invalid")
	}
	if err := route.beginAttach(); err != nil {
		return nil, err
	}
	defer route.endAttach()
	attachmentCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(route.doneCtx, cancel)
	defer func() { stop(); cancel() }()

	view, err := route.config.View.Current()
	if err != nil {
		return nil, err
	}
	identifier, selected, deadline, err := route.reserve(view, intent.Deadline)
	if err != nil {
		return nil, err
	}
	attachment, err := openNativeAttachment(attachmentCtx, route.config.Entry, selected, identifier, deadline,
		route.config.Admit, func() { route.release(identifier) })
	if err != nil {
		route.release(identifier)
		return nil, err
	}
	route.mu.Lock()
	if route.closed {
		delete(route.pending, identifier)
		route.mu.Unlock()
		return nil, errors.Join(errors.New("native Route is closed"), attachment.Close())
	}
	delete(route.pending, identifier)
	route.active[identifier] = activeAttachment{plan: selected, attachment: attachment}
	route.mu.Unlock()
	return attachment, nil
}

// Close cancels pending attachment attempts, joins them, and attempts to close
// every active attachment. A nil result proves that no Route selection or
// resource reservation remains live; otherwise it reports the joined cleanup
// evidence without claiming successful release.
func (route *Route) Close() error {
	if route == nil {
		return errors.New("native Route is unavailable")
	}
	route.mu.Lock()
	if route.closed {
		route.mu.Unlock()
		return errors.New("native Route is closed")
	}
	route.closed = true
	route.done()
	for route.flight != 0 {
		route.idle.Wait()
	}
	attachments := make([]*Attachment, 0, len(route.active))
	for _, active := range route.active {
		attachments = append(attachments, active.attachment)
	}
	route.mu.Unlock()
	var result error
	for _, attachment := range attachments {
		result = errors.Join(result, attachment.Close())
	}
	return result
}

func (route *Route) beginAttach() error {
	route.mu.Lock()
	defer route.mu.Unlock()
	if route.closed {
		return errors.New("native Route is closed")
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

func (route *Route) reserve(view state.Snapshot, requested time.Time) ([32]byte, plan, time.Time, error) {
	var identifier, seed [32]byte
	if _, err := rand.Read(identifier[:]); err != nil {
		return [32]byte{}, plan{}, time.Time{}, err
	}
	if _, err := rand.Read(seed[:]); err != nil {
		return [32]byte{}, plan{}, time.Time{}, err
	}
	if view.TrustedTime.IsZero() {
		return [32]byte{}, plan{}, time.Time{}, errors.New("Route View has no trusted decision time")
	}
	route.mu.Lock()
	defer route.mu.Unlock()
	if route.closed {
		return [32]byte{}, plan{}, time.Time{}, errors.New("native Route is closed")
	}
	input := selection{seed: seed, at: view.TrustedTime.UTC()}
	for _, selected := range route.pending {
		input = excludePlan(input, selected)
	}
	for _, active := range route.active {
		input = excludePlan(input, active.plan)
	}
	selected, err := selectRoute(view, input)
	if err != nil {
		return [32]byte{}, plan{}, time.Time{}, err
	}
	deadline := selected.deadline(requested)
	if !view.TrustedTime.Before(deadline) {
		return [32]byte{}, plan{}, time.Time{}, errors.New("native Route safety deadline elapsed")
	}
	route.pending[identifier] = selected
	return identifier, selected, deadline, nil
}

func excludePlan(input selection, selected plan) selection {
	for _, position := range selected.positions {
		input.excludedIdentities = append(input.excludedIdentities, position.nodeID)
		input.excludedFamilies = append(input.excludedFamilies, position.family)
	}
	return input
}

func (selected plan) deadline(requested time.Time) time.Time {
	deadline := requested.UTC()
	if selected.validUntil.Before(deadline) {
		deadline = selected.validUntil
	}
	for _, position := range selected.positions {
		if position.validUntil.Before(deadline) {
			deadline = position.validUntil
		}
		if position.assignmentNotAfter.Before(deadline) {
			deadline = position.assignmentNotAfter
		}
	}
	return deadline
}

func (route *Route) release(identifier [32]byte) {
	route.mu.Lock()
	delete(route.pending, identifier)
	delete(route.active, identifier)
	route.mu.Unlock()
}
