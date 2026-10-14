//go:build linux && text_worker_installed

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/successor/connection"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	publicationruntime "github.com/dianabuilds/ardents-network/internal/successor/publication/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// Publisher JOIN starts only after its original delivery completed RESULT/CLOSE.
// Hold that genuine observation before its channel opens. Holding the earlier
// Introduction CLOSE would instead cancel an unfinished delivery when the
// concurrent Reader JOIN fails, conflating the RESULT and JOIN cuts.
func (g *publicationDeliveryCloseGate) observePublisherJoin(current func() (network.RuntimeView, error)) (network.RuntimeView, error) {
	var callers [64]uintptr
	frames := runtime.CallersFrames(callers[:runtime.Callers(2, callers[:])])
	joinOpening, publisher := false, false
	for {
		frame, more := frames.Next()
		joinOpening = joinOpening || frame.Function == "github.com/dianabuilds/ardents-network/internal/successor/route/join.(*JoinAcquisition).Join"
		publisher = publisher || frame.Function == "github.com/dianabuilds/ardents-network/internal/successor/publication/runtime.(*Publisher).NextAuthentication"
		if !more {
			break
		}
	}
	if joinOpening && publisher {
		g.mu.Lock()
		var resume <-chan struct{}
		if g.armed {
			g.once.Do(func() { close(g.entered) })
			resume = g.resume
		}
		g.mu.Unlock()
		if resume != nil {
			<-resume
		}
	}
	return current()
}

// This component uses qualified original workers and actual Route receivers.
// It is not acceptance of two independently installed Endpoint managers.
func TestInstalledConnectionNonceBothCarriers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			t.Run("result", func(t *testing.T) { runInstalledConnectionAuthenticationFailure(t, carrier, "result") })
			t.Run("join", func(t *testing.T) { runInstalledConnectionAuthenticationFailure(t, carrier, "join") })
			t.Run("tls", func(t *testing.T) { runInstalledConnectionAuthenticationFailure(t, carrier, "tls") })
		})
	}
}

func TestInstalledConnectionWithdrawalBothCarriers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			runInstalledConnectionTextLoss(t, carrier, "publication-withdrawal")
		})
	}
}

func assertInstalledConnectionPublicationWithdrawal(t *testing.T, ctx context.Context, publisher *publicationruntime.Publisher, publisherOperation, readerOperation *executionruntime.Operation, peer, stream *connection.Stream) {
	t.Helper()
	if n, err := peer.Write([]byte("original bytes")); err != nil || n != len("original bytes") {
		t.Fatal("original accepting Publisher bytes", n, err)
	}
	buffer := make([]byte, len("original bytes"))
	if _, err := io.ReadFull(stream, buffer); err != nil || string(buffer) != "original bytes" {
		t.Fatal("original authenticated bytes before withdrawal", err)
	}
	withdrawErr := publisher.Withdraw(ctx)
	if withdrawErr != nil && !onlyInstalledConnectionCancellation(withdrawErr) {
		t.Fatal("Publication withdrawal retained an unexpected failure", withdrawErr)
	}
	if physical, joined := publisher.CleanupResult(); !joined || physical != nil {
		t.Fatal("withdrawn Publication did not join original physical borrowers", physical, joined, withdrawErr)
	}
	if err := errors.Join(publisherOperation.CheckPublisher(), readerOperation.CheckReader()); err != nil {
		t.Fatal("Publication withdrawal changed original Jobs", err)
	}
	if n, err := peer.Write([]byte("late")); n != 0 || err == nil {
		t.Fatal("withdrawn Publisher accepted Application bytes", n, err)
	}
	contract, err := publisherTextContract(publisherOperation)
	if err != nil {
		t.Fatal(err)
	}
	if acquired, err := publisher.NextStream(ctx, contract); acquired != nil || err == nil {
		if acquired != nil {
			_ = acquired.Close()
		}
		t.Fatal("withdrawn Publication acquired another Connection", err)
	}
	buffer = []byte{0xa5}
	if n, err := stream.Read(buffer); n != 0 || err == nil || err == io.EOF || buffer[0] != 0xa5 {
		t.Fatal("withdrawn Publication became bytes or authenticated EOF", n, err)
	}
	closeErr := stream.Close()
	if closeErr == nil || stream.JoinedCleanup() != nil {
		t.Fatal("Reader lost withdrawal refusal or original physical join", closeErr, stream.JoinedCleanup())
	}
	if repeated := stream.Close(); repeated != closeErr {
		t.Fatal("Reader withdrawal changed its retained Close result", repeated, closeErr)
	}
	if err := errors.Join(publisherOperation.CheckPublisher(), readerOperation.CheckReader()); err != nil {
		t.Fatal("Connection retirement changed original Jobs", err)
	}
}

func TestInstalledConnectionRefreshBothCarriers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			t.Run("accepted", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "refresh-response") })
			t.Run("pending-ack", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "refresh-pending") })
		})
	}
}

func TestInstalledConnectionStreamBothCarriers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			t.Run("before-request-loss", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "before-request") })
			t.Run("before-request-terminal-commit-loss", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "terminal-body") })
			t.Run("responder-parent-loss", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "responder-parent") })
			t.Run("source-parent-loss", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "source-parent") })
			t.Run("source-stock-exhaustion", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "source-stock-exhaustion") })
			t.Run("publisher-source-parent-loss", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "publisher-source-parent") })
			t.Run("response-loss", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "response") })
			t.Run("after-request-terminal-loss", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "request-terminal") })
			t.Run("caller-observation-loss", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "caller-observation") })
			t.Run("close-observation-loss", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "close-observation") })
			t.Run("publisher-observation-loss", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "publisher-observation") })
			t.Run("publisher-close-observation-loss", func(t *testing.T) { runInstalledConnectionTextLoss(t, carrier, "publisher-close-observation") })
		})
	}
}

// These controls retain genuine workers and one original Application request.
func TestInstalledConnectionControlWritesBothCarriers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, role := range []string{"initiator", "publisher"} {
				t.Run(role, func(t *testing.T) {
					for _, kind := range []string{"terminal", "receipt", "confirmation"} {
						t.Run(kind, func(t *testing.T) {
							for _, cut := range []string{"before-write", "after-write"} {
								t.Run(cut, func(t *testing.T) {
									runInstalledConnectionTextLoss(t, carrier, "control-"+role+"-"+kind+"-"+cut)
								})
							}
						})
					}
				})
			}
		})
	}
}

// The writer remains inside its successful physical Write while the opposite
// reader proves it has accepted that exact generation's control. Retirement
// therefore occurs after peer acceptance but before the sender's finishWrite.
func TestInstalledConnectionControlReceivedBothCarriers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, role := range []string{"initiator", "publisher"} {
				t.Run(role, func(t *testing.T) {
					for _, kind := range []string{"terminal", "receipt", "confirmation"} {
						t.Run(kind, func(t *testing.T) {
							runInstalledConnectionTextLoss(t, carrier, "control-"+role+"-"+kind+"-after-receive")
						})
					}
				})
			}
		})
	}
}

func TestInstalledConnectionControlDeliveryBothCarriers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, role := range []string{"initiator", "publisher"} {
				t.Run(role, func(t *testing.T) {
					for _, kind := range []string{"terminal", "receipt", "confirmation"} {
						t.Run(kind, func(t *testing.T) {
							for _, cut := range []string{"before-receive", "after-finish"} {
								t.Run(cut, func(t *testing.T) {
									runInstalledConnectionTextLoss(t, carrier, "control-"+role+"-"+kind+"-"+cut)
								})
							}
						})
					}
				})
			}
		})
	}
}

// These source sites are checked by the architecture profile: a moved call
// cannot silently retarget a qualification fault to a prefix or pre-read check.
const connectionControlFramingReadLine = 36
const connectionControlReadRecheckLine = 381

const connectionPublisherControlWriteBeforeLine = 385
const connectionPublisherControlWriteAfterLine = 397
const connectionInitiatorControlWriteBeforeLine = 127
const connectionInitiatorControlWriteAfterLine = 132

// This observer runs on the actual serialized record writer, inside its
// existing authority callback. The writer owns its immutable active ticket
// until finishWrite; replacement cannot change terminal until that writer has
// joined. Peer readers update receipt flags, not this ticket or its record.
// Reflection reads only that writer's own immutable metadata, never authority,
// secrets or concurrently updated ledger flags. No supplied record/stream or
// accepting callback enters production. A generation-1 ticket selects this
// exact initial control even if Data ACKs interleave with it.
type installedControlWriteGate struct {
	mu            sync.Mutex
	target        *connection.Stream
	role          string
	kind          string
	after         bool
	entered       chan struct{}
	resume        chan struct{}
	once          sync.Once
	release       sync.Once
	receiver      *connection.Stream
	received      chan struct{}
	receiveOnce   sync.Once
	beforeReceive bool
	afterFinish   bool
	writing       chan struct{}
	writingOnce   sync.Once
}

// Only the opposite original record reader examines these fields, which it
// alone writes. The writer does not mutate them; replacement must join this
// reader before replacing the terminal ledger. No mutable writer flags or
// private authority are read, and no record or accepting result is supplied.
func (gate *installedControlWriteGate) observeReceived() {
	var pcs [96]uintptr
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs[:])])
	reader := false
	for {
		frame, more := frames.Next()
		if frame.Function == "github.com/dianabuilds/ardents-network/internal/successor/connection.(*Stream).readRecords" {
			reader = true
		}
		if !more {
			break
		}
	}
	if !reader {
		return
	}
	if gate.beforeReceive {
		select {
		case <-gate.writing:
		default:
			return
		}
	}
	gate.mu.Lock()
	selected := false
	if gate.receiver != nil {
		terminal := reflect.ValueOf(gate.receiver).Elem().FieldByName("terminal")
		if !terminal.IsNil() && terminal.Elem().FieldByName("generation").Uint() == 1 {
			field := "remoteClosed"
			if gate.kind == "receipt" {
				field = "receiptReceived"
			}
			if gate.kind == "confirmation" {
				field = "receiptConfirmed"
			}
			selected = terminal.Elem().FieldByName(field).Bool()
			if gate.beforeReceive {
				selected = !selected
			}
		}
	}
	if selected {
		gate.receiveOnce.Do(func() { close(gate.received) })
	}
	gate.mu.Unlock()
	if selected {
		<-gate.resume
	}
}

func (gate *installedControlWriteGate) arm(target *connection.Stream) {
	gate.mu.Lock()
	gate.target = target
	gate.mu.Unlock()
}

func (gate *installedControlWriteGate) unblock() {
	gate.release.Do(func() { close(gate.resume) })
}

func (gate *installedControlWriteGate) observe() {
	var pcs [96]uintptr
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs[:])])
	actor, boundary, before, direct, source := false, false, false, false, false
	responderBorrow, physicalDepth := false, 0
	for {
		frame, more := frames.Next()
		switch frame.Function {
		case "github.com/dianabuilds/ardents-network/internal/successor/connection.(*Stream).writeRecords":
			actor = true
		case "github.com/dianabuilds/ardents-network/internal/successor/publication/instance.(*ServiceAttachment).Write":
			if gate.role == "publisher" {
				line := connectionPublisherControlWriteBeforeLine
				if gate.after {
					line = connectionPublisherControlWriteAfterLine
				}
				boundary = frame.Line == line
				before = frame.Line == connectionPublisherControlWriteBeforeLine
			}
		case "github.com/dianabuilds/ardents-network/internal/successor/connection.(*initiatorAttachment).Write":
			if gate.role == "initiator" {
				line := connectionInitiatorControlWriteBeforeLine
				if gate.after {
					line = connectionInitiatorControlWriteAfterLine
				}
				boundary = frame.Line == line
				before = frame.Line == connectionInitiatorControlWriteBeforeLine
			}
		case "github.com/dianabuilds/ardents-network/internal/successor/route/prefix.(*ResponderScope).Check":
			direct = true
		case "github.com/dianabuilds/ardents-network/internal/successor/route/prefix.(*ResponderJoinBorrow).CheckOriginal":
			responderBorrow = true
		case "github.com/dianabuilds/ardents-network/internal/successor/route/prefix.(*Prefix).originalCurrent":
			physicalDepth++
		case "github.com/dianabuilds/ardents-network/internal/successor/route/prefix.(*SourceScope).Check":
			source = true
		case "github.com/dianabuilds/ardents-network/internal/successor/route/join.(*Joined).CheckSourceCaller":
			source = true
		}
		if !more {
			break
		}
	}
	// The scoped borrow now observes the physical Responder directly. Its
	// nested Source observation has two originalCurrent frames and must not
	// select this same write a second time.
	direct = direct || responderBorrow && physicalDepth == 1
	if !actor || !(boundary || gate.beforeReceive && before || gate.afterFinish) || gate.role == "publisher" && (!direct || source) || gate.role == "initiator" && !source {
		return
	}
	gate.mu.Lock()
	selected := false
	if gate.target != nil {
		terminal := reflect.ValueOf(gate.target).Elem().FieldByName("terminal")
		if !terminal.IsNil() && gate.afterFinish {
			// Only this original writer changes completed-write flags. A
			// replacement cannot modify its ledger before this actor joins.
			field := "terminalWritten"
			if gate.kind == "receipt" {
				field = "receiptWritten"
			}
			if gate.kind == "confirmation" {
				field = "confirmationWritten"
			}
			selected = terminal.Elem().FieldByName("generation").Uint() == 1 && terminal.Elem().FieldByName(field).Bool()
		} else if !terminal.IsNil() {
			active := terminal.Elem().FieldByName("active")
			if !active.IsNil() && active.Elem().FieldByName("generation").Uint() == 1 {
				record := active.Elem().FieldByName("record")
				kind := "terminal"
				if record.FieldByName("Terminal").IsNil() {
					ack := record.FieldByName("Acknowledgement")
					if !ack.IsNil() && ack.Elem().FieldByName("Terminal").Bool() {
						kind = "receipt"
						if ack.Elem().FieldByName("TerminalConfirmation").Bool() {
							kind = "confirmation"
						}
					} else {
						kind = ""
					}
				}
				selected = kind == gate.kind
			}
		}
	}
	if selected && gate.beforeReceive && before {
		// Announce the actual selected ticket before its physical Write.
		// The opposite reader can then stop before accepting it; the
		// coordinator separately waits for successful physical output.
		gate.writingOnce.Do(func() { close(gate.writing) })
		selected = false
	}
	if selected {
		gate.once.Do(func() { close(gate.entered) })
	}
	gate.mu.Unlock()
	if selected {
		<-gate.resume
	}
}

// One fixed Text request writes one Data record followed by its Terminal.
// Each is a separate small TLS record, read through the Attachment's framing
// scratch. Select the Publisher's second framing fill, after real TLS I/O and
// before the original post-read authority check returns. The scheduler returns
// no authority, record or error; the coordinator stops the actual Rendezvous.
func installedPublisherTerminalBodyDelay() (observe func(), release func(), entered <-chan struct{}) {
	var mu sync.Mutex
	reads := 0
	arrival, resume := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	observe = func() {
		var pcs [96]uintptr
		frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs[:])])
		body, post, scope, source, actor := false, false, false, false, false
		responderBorrow, physicalDepth := false, 0
		for {
			frame, more := frames.Next()
			switch frame.Function {
			case "github.com/dianabuilds/ardents-network/internal/successor/connection.(*attachmentRecordReader).Read":
				body = frame.Line == connectionControlFramingReadLine
			case "github.com/dianabuilds/ardents-network/internal/successor/publication/instance.(*ServiceAttachment).Read":
				post = frame.Line == connectionControlReadRecheckLine
			case "github.com/dianabuilds/ardents-network/internal/successor/route/prefix.(*ResponderScope).Check":
				scope = true
			case "github.com/dianabuilds/ardents-network/internal/successor/route/prefix.(*ResponderJoinBorrow).CheckOriginal":
				responderBorrow = true
			case "github.com/dianabuilds/ardents-network/internal/successor/route/prefix.(*Prefix).originalCurrent":
				physicalDepth++
			case "github.com/dianabuilds/ardents-network/internal/successor/route/prefix.(*SourceScope).Check":
				source = true
			case "github.com/dianabuilds/ardents-network/internal/successor/connection.(*Stream).readRecords":
				actor = true
			}
			if !more {
				break
			}
		}
		scope = scope || responderBorrow && physicalDepth == 1
		// Only the direct Responder observation counts once per read; its
		// separate nested Source observation must not count the same body twice.
		if !body || !post || !scope || source || !actor {
			return
		}
		mu.Lock()
		reads++
		selected := reads == 2
		if selected {
			close(arrival)
		}
		mu.Unlock()
		if selected {
			<-resume
		}
	}
	release = func() { releaseOnce.Do(func() { close(resume) }) }
	return observe, release, arrival
}

// Fork only the actual published outcome for the local fixture observer. This
// supplies no outcome, authority or accepting state of its own; the original
// Connection still performs every effect, final fence and physical join.
type observedConnectionOutcome struct {
	*connection.Stream
	consumer chan application.Outcome
	observed chan application.Outcome
}

func observeConnectionOutcome(stream *connection.Stream) *observedConnectionOutcome {
	observer := &observedConnectionOutcome{Stream: stream, consumer: make(chan application.Outcome, 1), observed: make(chan application.Outcome, 1)}
	go func() {
		if outcome, ok := <-stream.Done(); ok {
			observer.consumer <- outcome
			observer.observed <- outcome
		}
		close(observer.consumer)
		close(observer.observed)
	}()
	return observer
}

func (observer *observedConnectionOutcome) Done() <-chan application.Outcome {
	return observer.consumer
}

func (observer *observedConnectionOutcome) waitClean(ctx context.Context) error {
	select {
	case outcome, ok := <-observer.observed:
		if !ok || outcome.Class != application.CleanClose {
			return errors.New("original Publisher Terminal outcome is not clean")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Delay only the return of an actual successful signed Network observation.
// No synthetic RuntimeView, permit or physical result is supplied. The gate
// has no owner state lock, and cleanup releases every original observer.
func installedConnectionObservationDelay() (observe, arm, release func(), entered <-chan struct{}) {
	var mu sync.Mutex
	armed := false
	arrived := make(chan struct{})
	resume := make(chan struct{})
	var arriveOnce, releaseOnce sync.Once
	observe = func() {
		// Select the synchronous Application Read's genuine observation,
		// rather than an unrelated background record actor. Stack inspection
		// controls test scheduling only; it supplies no authority or result.
		var callers [64]uintptr
		frames := runtime.CallersFrames(callers[:runtime.Callers(2, callers[:])])
		applicationRead := false
		for {
			frame, more := frames.Next()
			if frame.Function == "github.com/dianabuilds/ardents-network/internal/successor/connection.(*Stream).Read" {
				applicationRead = true
				break
			}
			if !more {
				break
			}
		}
		if !applicationRead {
			return
		}
		mu.Lock()
		wait := armed
		mu.Unlock()
		if wait {
			arriveOnce.Do(func() { close(arrived) })
			<-resume
		}
	}
	arm = func() { mu.Lock(); armed = true; mu.Unlock() }
	release = func() { releaseOnce.Do(func() { close(resume) }) }
	return observe, arm, release, arrived
}

// A refusal scenario retains its original cancellation rather than requiring
// successful Application cleanup. Exact peer-read retirement is also a work
// refusal; both original workers' physical receipts are checked separately.
// Other leaf errors still fail; containing cancellation alone is insufficient.
func retainInstalledConnectionCancellation(t *testing.T, err error, expected bool) bool {
	t.Helper()
	if !expected || !onlyInstalledConnectionCancellation(err) {
		return false
	}
	t.Log("retained original caller-loss cleanup", err)
	return true
}

// Only the exact previously observed joined logical refusal can be expected.
// Other leaves still fail; actual physical receipts are checked independently.
func onlyInstalledConnectionExpectedFailure(err, expected error) bool {
	if expected == nil {
		return false
	}
	if err == expected || onlyInstalledConnectionCancellation(err) {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if !onlyInstalledConnectionExpectedFailure(child, expected) {
				return false
			}
		}
		return len(joined.Unwrap()) != 0
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return onlyInstalledConnectionExpectedFailure(wrapped.Unwrap(), expected)
	}
	return false
}

func onlyInstalledConnectionCancellation(err error) bool {
	if err == context.Canceled {
		return true
	}
	if err == io.EOF || transport.IsPeerRetirementCause(err) {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !onlyInstalledConnectionCancellation(child) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return onlyInstalledConnectionCancellation(wrapped.Unwrap())
	}
	return false
}

func assertInstalledConnectionDelayedObservationRetirement(t *testing.T, ctx context.Context, cancelOriginal context.CancelFunc, checkOriginal func() error, stream, peer *connection.Stream, arm, release func(), entered <-chan struct{}, closeConnection bool) {
	t.Helper()
	defer release()
	// Actual authenticated Data establishes a positive byte oracle before the
	// fault. These are direct bounded Stream effects, not a completed Text Job.
	if n, err := peer.Write([]byte("original bytes")); err != nil || n != len("original bytes") {
		t.Fatal("original Publisher byte admission", n, err)
	}
	first := []byte{0}
	if n, err := stream.Read(first); err != nil || n != 1 || first[0] != 'o' {
		t.Fatal("original authenticated byte before authority loss", n, err)
	}
	arm()
	type readResult struct {
		n   int
		err error
	}
	read := make(chan readResult, 1)
	buffer := []byte{0xa5}
	go func() { n, err := stream.Read(buffer); read <- readResult{n, err} }()
	readJoined := false
	defer func() {
		cancelOriginal()
		release()
		if !readJoined {
			<-read
		}
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("original Network observation was not delayed", ctx.Err())
	}
	select {
	case result := <-read:
		readJoined = true
		t.Fatal("Read bypassed the delayed original observation", result.n, result.err)
	default:
	}
	// A second real operation on this same authenticated Stream must enter
	// its local state barrier while the first native observation is withheld.
	// An empty Write enters the ordinary original-owner gate without emitting
	// Application bytes or adding a terminal-control shutdown race.
	// The old observation-under-Stream.mu ordering blocks this Write.
	gateEntered := make(chan error, 1)
	go func() {
		n, err := stream.Write(nil)
		if n != 0 {
			err = errors.Join(err, errors.New("empty Write emitted Application bytes"))
		}
		gateEntered <- err
	}()
	select {
	case err := <-gateEntered:
		if err != nil {
			t.Fatal("original empty Write refused before caller loss", err)
		}
	case <-time.After(5 * time.Second):
		cancelOriginal()
		release()
		<-gateEntered
		t.Fatal("foreign observation held the Stream state barrier")
	}
	closed := make(chan error, 1)
	if closeConnection {
		go func() { closed <- stream.Close() }()
	} else {
		cancelOriginal()
		if checkOriginal() == nil {
			t.Fatal("revoked original operation still admitted effects")
		}
	}
	select {
	case <-stream.Context().Done():
	case <-time.After(5 * time.Second):
		t.Fatal("foreign observation prevented original logical cancellation")
	}
	if closeConnection {
		if err := checkOriginal(); err != nil {
			t.Fatal("Connection Close replaced or retired its original Job", err)
		}
	} else {
		go func() { closed <- stream.Close() }()
	}
	release()
	result := <-read
	readJoined = true
	if result.n != 0 || result.err == nil || buffer[0] != 0xa5 {
		t.Fatal("late original observation exposed bytes after Stream retirement", result.n, result.err, buffer[0])
	}
	closeErr := <-closed
	if !errors.Is(closeErr, context.Canceled) {
		t.Fatal("original canceled Stream lost its retained failure", closeErr)
	}
	if repeated := stream.Close(); repeated != closeErr {
		t.Fatal("repeated Close changed its original joined result", repeated, closeErr)
	}
	for outcome := range stream.Done() {
		if outcome.Class != application.IndeterminateFailure {
			t.Fatal("canceled original Stream supplied a successful outcome", outcome.Class)
		}
		t.Log("retained canceled Stream outcome", outcome.Class, outcome.Reason)
	}
	if closeConnection {
		if err := checkOriginal(); err != nil {
			t.Fatal("late Connection cleanup retired the original Job", err)
		}
		// Retire the genuinely unused worker through its own original caller;
		// Connection Close itself cannot revoke or complete that Job.
		cancelOriginal()
	}
	if checkOriginal() == nil {
		t.Fatal("late observation restored original operation authority")
	}
	// Original worker and Source joins belong to the fixture cleanup. No sealed
	// ReaderResult or successful Application completion is manufactured here.
}

// Pause after the original authenticated Read has returned real bytes. Every
// method otherwise uses the original Stream; no authority, bytes, EOF or
// terminal outcome is supplied by this observation. The pause holds no owner
// barrier and Close/cancellation release it.
type observedConnectionRead struct {
	*connection.Stream
	ctx        context.Context
	first      chan int
	resume     chan struct{}
	firstOnce  sync.Once
	resumeOnce sync.Once
}

// Hold only the delivery of the original Stream's authenticated directional
// EOF to the Publisher worker. The actual parser and control writer continue;
// no EOF, receipt, permission or successful outcome is supplied by this gate.
type observedConnectionEOF struct {
	*connection.Stream
	outcome    *observedConnectionOutcome
	ctx        context.Context
	arrived    chan struct{}
	resume     chan struct{}
	firstOnce  sync.Once
	resumeOnce sync.Once
}

func (observed *observedConnectionEOF) Done() <-chan application.Outcome {
	return observed.outcome.Done()
}

func (observed *observedConnectionEOF) Read(body []byte) (int, error) {
	n, err := observed.Stream.Read(body)
	if err == io.EOF {
		observed.firstOnce.Do(func() {
			close(observed.arrived)
			select {
			case <-observed.resume:
			case <-observed.ctx.Done():
				err = errors.Join(err, observed.ctx.Err())
			}
		})
	}
	return n, err
}

func (observed *observedConnectionEOF) release() {
	observed.resumeOnce.Do(func() { close(observed.resume) })
}

func (observed *observedConnectionEOF) Close() error {
	observed.release()
	return observed.Stream.Close()
}

func deliverReaderAfterRequestTerminalLoss(t *testing.T, ctx context.Context, cancel context.CancelFunc, operation *executionruntime.Operation, contract executionruntime.StreamContract, stream *connection.Stream, observed *observedConnectionEOF, loseOriginal func()) (bool, *executionruntime.ReaderResult, error) {
	t.Helper()
	type delivery struct {
		transferred bool
		result      *executionruntime.ReaderResult
		err         error
	}
	completed := make(chan delivery, 1)
	go func() {
		transferred, result, err := operation.DeliverReaderStream(contract, stream)
		completed <- delivery{transferred, result, err}
	}()
	joined := false
	t.Cleanup(func() {
		observed.release()
		if !joined {
			cancel()
			outcome := <-completed
			if outcome.result != nil {
				outcome.result.Close()
			}
		}
	})
	select {
	case <-observed.arrived:
		// The original Publisher has read the actual request Terminal but
		// its worker has not yet received EOF or emitted the response.
	case outcome := <-completed:
		joined = true
		if outcome.result != nil {
			outcome.result.Close()
		}
		t.Fatal("original Reader completed before request Terminal loss", outcome.err)
	case <-ctx.Done():
		t.Fatal("original authenticated request Terminal was not observed", ctx.Err())
	}
	loseOriginal()
	observed.release()
	outcome := <-completed
	joined = true
	return outcome.transferred, outcome.result, outcome.err
}

func (observed *observedConnectionRead) Read(p []byte) (int, error) {
	n, err := observed.Stream.Read(p)
	if n > 0 {
		observed.firstOnce.Do(func() {
			observed.first <- n
			select {
			case <-observed.resume:
			case <-observed.ctx.Done():
				err = errors.Join(err, observed.ctx.Err())
			}
		})
	}
	return n, err
}

func (observed *observedConnectionRead) release() {
	observed.resumeOnce.Do(func() { close(observed.resume) })
}

func (observed *observedConnectionRead) Close() error {
	observed.release()
	return observed.Stream.Close()
}

func deliverReaderAfterResponseLoss(t *testing.T, ctx context.Context, cancel context.CancelFunc, operation *executionruntime.Operation, contract executionruntime.StreamContract, stream *connection.Stream, responseSize int, loseOriginal func()) (bool, *executionruntime.ReaderResult, error) {
	t.Helper()
	observed := &observedConnectionRead{Stream: stream, ctx: ctx, first: make(chan int, 1), resume: make(chan struct{})}
	type delivery struct {
		transferred bool
		result      *executionruntime.ReaderResult
		err         error
	}
	completed := make(chan delivery, 1)
	go func() {
		transferred, result, err := operation.DeliverReaderStream(contract, observed)
		completed <- delivery{transferred, result, err}
	}()
	joined := false
	t.Cleanup(func() {
		observed.release()
		if !joined {
			cancel()
			outcome := <-completed
			if outcome.result != nil {
				outcome.result.Close()
			}
		}
	})
	select {
	case n := <-observed.first:
		if n <= 0 || n >= responseSize {
			t.Fatal("loss did not follow an incomplete original response")
		}
	case outcome := <-completed:
		joined = true
		if outcome.result != nil {
			outcome.result.Close()
		}
		t.Fatal("original Reader completed before response loss", outcome.err)
	case <-ctx.Done():
		t.Fatal("original response did not start", ctx.Err())
	}
	loseOriginal()
	observed.release()
	outcome := <-completed
	joined = true
	return outcome.transferred, outcome.result, outcome.err
}
