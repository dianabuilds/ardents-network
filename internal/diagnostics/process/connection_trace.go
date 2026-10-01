package process

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ConnectionStage is a fixed local observation category, never a remote identity.
type ConnectionStage uint8

const (
	Admission ConnectionStage = iota + 1
	Activation
	WorkerLaunch
	WorkerOperation
	Introduction
	ServiceJoin
	ServiceAuthentication
	LocalRequest
	DocumentExchange
	ServiceClose
	WorkerClose
	CurrentOwner
	ApplicationResponse
	CallerJoin
	SessionRelease
	WorkerActivation
)

var connectionStages = [...]string{"", "admission", "activation", "worker-launch", "worker-operation", "introduction", "service-join", "service-authentication", "local-request", "document-exchange", "service-close", "worker-close", "current-owner", "application-response", "caller-join", "session-release", "worker-activation"}

const connectionRecordLimit = 64
const connectionCaptureWindow = 10 * time.Minute

type captureContextKey struct{}
type sessionCaptureContextKey struct{}

// ConnectionTrace retains only the first Reader operation in an explicitly
// enabled process session. Its zero and nil values are disabled. It owns no product work,
// deadlines, cancellation, authority or timers.
type ConnectionTrace struct {
	mu       sync.Mutex
	now      func() time.Time
	opened   time.Time
	started  time.Time
	claimed  bool
	terminal bool
	outcome  string
	lost     uint64
	records  []connectionRecord
}

type connectionRecord struct {
	Stage       string `json:"stage"`
	State       string `json:"state"`
	Elapsed     int64  `json:"elapsed_ns"`
	Duration    *int64 `json:"duration_ns,omitempty"`
	Budget      *int64 `json:"remaining_budget_ns,omitempty"`
	ContextStop string `json:"context_stop,omitempty"`
}

type connectionSnapshot struct {
	Schema  string             `json:"schema"`
	State   string             `json:"state"`
	Outcome string             `json:"outcome,omitempty"`
	Lost    uint64             `json:"lost_records"`
	Limit   int                `json:"record_limit"`
	Records []connectionRecord `json:"records"`
}

func newConnectionCapture() *ConnectionTrace {
	return &ConnectionTrace{now: time.Now, opened: time.Now(), records: make([]connectionRecord, 0, connectionRecordLimit)}
}

// ReaderTrace claims the first Reader operation from the configured process
// session. Later operations return nil, including after expiry or completion.
func ReaderTrace(ctx context.Context) *ConnectionTrace {
	if ctx == nil {
		return nil
	}
	capture, _ := ctx.Value(sessionCaptureContextKey{}).(*ConnectionTrace)
	if capture == nil || capture.now == nil {
		return nil
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if capture.claimed || capture.expiredLocked() {
		return nil
	}
	capture.claimed = true
	capture.started = capture.now()
	return capture
}

// Bind transfers the same observation handle onto an existing authorized
// Context after re-parenting. A disabled handle returns that exact Context.
func (capture *ConnectionTrace) Bind(ctx context.Context) context.Context {
	if capture == nil || capture.now == nil {
		return ctx
	}
	return context.WithValue(ctx, captureContextKey{}, capture)
}

// Observe records a fixed stage start and returns its once-only completion.
// It performs bounded memory work only. Error text is never retained.
func (capture *ConnectionTrace) Observe(stage ConnectionStage, ctx context.Context) func(error) {
	if capture == nil || capture.now == nil || stage < Admission || int(stage) >= len(connectionStages) {
		return ignoreConnectionOutcome
	}
	capture.mu.Lock()
	started := capture.now()
	record := connectionRecord{Stage: connectionStages[stage], State: "started", Elapsed: started.Sub(capture.started).Nanoseconds()}
	if ctx != nil {
		if deadline, ok := ctx.Deadline(); ok {
			remaining := deadline.Sub(started).Nanoseconds()
			if remaining < 0 {
				remaining = 0
			}
			record.Budget = &remaining
		}
	}
	capture.appendLocked(record)
	capture.mu.Unlock()
	var once sync.Once
	return func(err error) {
		once.Do(func() {
			capture.mu.Lock()
			defer capture.mu.Unlock()
			ended := capture.now()
			duration := ended.Sub(started).Nanoseconds()
			record := connectionRecord{Stage: connectionStages[stage], State: connectionOutcome(err), Elapsed: ended.Sub(capture.started).Nanoseconds(), Duration: &duration}
			if ctx != nil && ctx.Err() != nil {
				record.ContextStop = connectionOutcome(ctx.Err())
			}
			capture.appendLocked(record)
		})
	}
}

// ObserveConnection observes the capture already carried by ctx, without
// claiming another Reader operation or changing the Context.
func ObserveConnection(ctx context.Context, stage ConnectionStage) func(error) {
	return ConnectionTraceFor(ctx).Observe(stage, ctx)
}

// ConnectionTraceFor returns only the explicitly bound Reader handle, never the
// session's unclaimed capture. Owners use it to observe their actual bounded
// Context after re-parenting without changing that Context.
func ConnectionTraceFor(ctx context.Context) *ConnectionTrace {
	if ctx == nil {
		return nil
	}
	capture, _ := ctx.Value(captureContextKey{}).(*ConnectionTrace)
	return capture
}

func ignoreConnectionOutcome(error) {}

func connectionOutcome(err error) string {
	if err == nil {
		return "completed"
	}
	remaining := 64
	class, ok := connectionCancellation(err, &remaining)
	if ok {
		return class
	}
	return "failed"
}

func connectionCancellation(err error, remaining *int) (string, bool) {
	if *remaining == 0 {
		return "", false
	}
	*remaining = *remaining - 1
	if err == context.DeadlineExceeded {
		return "deadline", true
	}
	if err == context.Canceled {
		return "canceled", true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 || len(causes) > 64 {
			return "", false
		}
		outcome := "canceled"
		for _, cause := range causes {
			class, ok := connectionCancellation(cause, remaining)
			if !ok {
				return "", false
			}
			if class == "deadline" {
				outcome = class
			}
		}
		return outcome, true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return connectionCancellation(wrapped, remaining)
	}
	return "", false
}
func (capture *ConnectionTrace) expiredLocked() bool {
	return capture.now().Sub(capture.opened) >= connectionCaptureWindow
}

func (capture *ConnectionTrace) appendLocked(record connectionRecord) {
	if !capture.claimed || capture.terminal {
		return
	}
	if capture.expiredLocked() || len(capture.records) == connectionRecordLimit {
		capture.lost++
		return
	}
	capture.records = append(capture.records, record)
}

// Finish marks joined completion, after all owned cleanup and caller callbacks.
// Application EOF or Open returning a stream is not joined completion.
func (capture *ConnectionTrace) Finish(err error) {
	if capture == nil || capture.now == nil {
		return
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if !capture.claimed || capture.terminal {
		return
	}
	if capture.expiredLocked() {
		capture.lost++
	}
	capture.terminal = true
	capture.outcome = connectionOutcome(err)
}

func (capture *ConnectionTrace) snapshot() connectionSnapshot {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	result := connectionSnapshot{Schema: "ardents-reader-trace-v1", State: "waiting", Lost: capture.lost, Limit: connectionRecordLimit, Records: append([]connectionRecord{}, capture.records...)}
	if capture.claimed {
		result.State = "active"
	}
	if capture.terminal {
		result.State, result.Outcome = "joined", capture.outcome
	}
	if capture.expiredLocked() {
		result.State = "expired"
	}
	if capture.lost != 0 {
		result.State = "incomplete"
	}
	return result
}
