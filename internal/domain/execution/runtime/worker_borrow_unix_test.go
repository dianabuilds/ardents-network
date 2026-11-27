//go:build linux || darwin

package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/execution"
	"github.com/dianabuilds/ardents-network/internal/domain/execution/worker"
)

// borrowFixture builds a live local Invocation/Operation with no installed
// worker: a genuine admitted Job and lease plus a zero attachment whose physical
// I/O refuses and whose Close joins nil. It supplies no worker confinement or
// successful physical cleanup evidence; the installed Reader and Stream
// qualification suites own that real physical path.
type borrowFixture struct {
	invocation   *Invocation
	operation    *Operation
	job          *execution.Job
	readerCaller context.Context
	cancelReader context.CancelFunc
}

func newBorrowFixture(t *testing.T) *borrowFixture {
	t.Helper()
	authority, err := execution.New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: [32]byte{2}, Surface: execution.Connection}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(authority.Close)
	supervisor, err := execution.NewSupervisor(authority)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = supervisor.Close() })
	capability, err := authority.Admit([32]byte{2}, execution.Connection)
	if err != nil {
		t.Fatal(err)
	}
	session, err := supervisor.Activate(t.Context(), capability, [32]byte{2}, execution.Connection)
	if err != nil {
		t.Fatal(err)
	}
	job, err := session.BeginJob()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { job.Retire(); _ = job.Finish(nil) })
	leaseCapability, err := authority.Admit([32]byte{2}, execution.Connection)
	if err != nil {
		t.Fatal(err)
	}
	lease, _, err := authority.Activate(t.Context(), leaseCapability, [32]byte{2}, execution.Connection)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lease.Release)
	readerCaller, cancelReader := context.WithCancel(t.Context())
	t.Cleanup(cancelReader)
	invocation := &Invocation{
		job:          job,
		activation:   &worker.Activation{Attachment: &worker.Attachment{}},
		readerCaller: readerCaller,
		readerInput:  &readerConnectionInput{},
	}
	operation := &Operation{
		invocation: invocation,
		job:        job,
		lease:      lease,
		ctx:        t.Context(),
		caller:     t.Context(),
		surface:    execution.Connection,
	}
	operation.self = operation
	invocation.operation = operation
	return &borrowFixture{invocation: invocation, operation: operation, job: job, readerCaller: readerCaller, cancelReader: cancelReader}
}

// streamBorrow and readerBorrow mirror the exact production construction: the
// shared mechanism with each role's distinct check hook and unavailable prose.
func (fixture *borrowFixture) streamBorrow() *workerBorrow {
	return &workerBorrow{invocation: fixture.invocation, unavailable: "original qualification operation is unavailable", check: streamOperationCheck}
}

func (fixture *borrowFixture) readerBorrow() *workerBorrow {
	return &workerBorrow{invocation: fixture.invocation, unavailable: "original Reader operation is unavailable", check: readerOperationCheck}
}

func borrowErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// The shared mechanism must produce identical Read/Write/Close mechanics for
// both roles on independent live fixtures. With a live Job and operation and a
// zero attachment, both delegate to the attachment and join nil authority, so
// the physical refusal is the sole result and Close joins nil twice.
func TestWorkerBorrowSharedMechanismEquivalence(t *testing.T) {
	type observation struct {
		readN         int
		readErr       string
		writeN        int
		writeErr      string
		closeErr      string
		closeAgainErr string
	}
	run := func(borrow *workerBorrow) observation {
		var observed observation
		n, err := borrow.Read(make([]byte, 4))
		observed.readN, observed.readErr = n, borrowErrString(err)
		n, err = borrow.Write([]byte("body"))
		observed.writeN, observed.writeErr = n, borrowErrString(err)
		observed.closeErr = borrowErrString(borrow.Close())
		observed.closeAgainErr = borrowErrString(borrow.Close())
		return observed
	}
	stream := run(newBorrowFixture(t).streamBorrow())
	reader := run(newBorrowFixture(t).readerBorrow())
	if stream != reader {
		t.Fatalf("shared worker borrow diverged by role: stream=%+v reader=%+v", stream, reader)
	}
	if stream.readN != 0 || stream.readErr != "document worker attachment is unavailable" {
		t.Fatalf("Read did not delegate and join identically: %+v", stream)
	}
	if stream.writeN != 0 || stream.writeErr != "document worker attachment is unavailable" {
		t.Fatalf("Write did not delegate and join identically: %+v", stream)
	}
	if stream.closeErr != "" || stream.closeAgainErr != "" {
		t.Fatalf("Close did not join nil authority and physical identically: %+v", stream)
	}
}

// Read gates on the original Job before any attachment I/O, identically for
// both roles.
func TestWorkerBorrowReadGatesOnJob(t *testing.T) {
	streamFixture, readerFixture := newBorrowFixture(t), newBorrowFixture(t)
	streamFixture.job.Retire()
	readerFixture.job.Retire()
	streamN, streamErr := streamFixture.streamBorrow().Read(make([]byte, 4))
	readerN, readerErr := readerFixture.readerBorrow().Read(make([]byte, 4))
	if streamN != 0 || readerN != 0 {
		t.Fatal("retired Job delivered attachment bytes")
	}
	if streamErr == nil || readerErr == nil || streamErr.Error() != readerErr.Error() {
		t.Fatalf("Read Job gate diverged by role: %v vs %v", streamErr, readerErr)
	}
	if streamErr.Error() != "execution Job is retired" {
		t.Fatalf("Read did not gate on the original Job: %v", streamErr)
	}
}

// The unavailable prose stays byte-identical to the original per-role copies and
// the two roles never share it.
func TestWorkerBorrowUnavailableProse(t *testing.T) {
	const streamProse = "original qualification operation is unavailable"
	const readerProse = "original Reader operation is unavailable"
	if streamProse == readerProse {
		t.Fatal("role unavailable prose merged")
	}
	for _, test := range []struct {
		name   string
		mutate func(*borrowFixture)
	}{
		{"absent operation", func(fixture *borrowFixture) { fixture.invocation.operation = nil }},
		{"foreign operation", func(fixture *borrowFixture) { fixture.operation.invocation = &Invocation{} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			streamFixture, readerFixture := newBorrowFixture(t), newBorrowFixture(t)
			test.mutate(streamFixture)
			test.mutate(readerFixture)
			if err := streamFixture.streamBorrow().checkOperation(); err == nil || err.Error() != streamProse {
				t.Fatalf("Stream qualification unavailable prose drifted: %v", err)
			}
			if err := readerFixture.readerBorrow().checkOperation(); err == nil || err.Error() != readerProse {
				t.Fatalf("Reader unavailable prose drifted: %v", err)
			}
		})
	}
}

// Write gates on the per-role operation check before any attachment I/O, so an
// unavailable operation returns that role's exact prose without writing.
func TestWorkerBorrowWriteGatesOnOperation(t *testing.T) {
	streamFixture, readerFixture := newBorrowFixture(t), newBorrowFixture(t)
	streamFixture.invocation.operation = nil
	readerFixture.invocation.operation = nil
	streamN, streamErr := streamFixture.streamBorrow().Write([]byte("body"))
	if streamN != 0 || streamErr == nil || streamErr.Error() != "original qualification operation is unavailable" {
		t.Fatalf("Stream Write did not gate on the original operation: %d %v", streamN, streamErr)
	}
	readerN, readerErr := readerFixture.readerBorrow().Write([]byte("body"))
	if readerN != 0 || readerErr == nil || readerErr.Error() != "original Reader operation is unavailable" {
		t.Fatalf("Reader Write did not gate on the original operation: %d %v", readerN, readerErr)
	}
}

// Close captures final authority before physical retirement and retains the
// first result through once, identically for both roles.
func TestWorkerBorrowCloseRetainsFirstAuthority(t *testing.T) {
	for _, role := range []struct {
		name   string
		prose  string
		borrow func(*borrowFixture) *workerBorrow
	}{
		{"stream", "original qualification operation is unavailable", (*borrowFixture).streamBorrow},
		{"reader", "original Reader operation is unavailable", (*borrowFixture).readerBorrow},
	} {
		t.Run(role.name, func(t *testing.T) {
			fixture := newBorrowFixture(t)
			borrow := role.borrow(fixture)
			fixture.invocation.operation = nil
			err := borrow.Close()
			if err == nil || err.Error() != role.prose {
				t.Fatalf("Close lost final authority: %v", err)
			}
			if again := borrow.Close(); again == nil || again.Error() != role.prose {
				t.Fatalf("Close was not idempotent through once: %v", again)
			}
		})
	}
}

// The per-role check hooks remain distinct: cancelling the reader caller fails
// only the Reader hook, never the Stream qualification hook. Possession merges
// no authority, and the Reader borrow must return that cancellation on Read.
func TestWorkerBorrowRoleChecksAreDistinct(t *testing.T) {
	fixture := newBorrowFixture(t)
	stream, reader := fixture.streamBorrow(), fixture.readerBorrow()
	if err := stream.checkOperation(); err != nil {
		t.Fatalf("Stream qualification check failed while live: %v", err)
	}
	if err := reader.checkOperation(); err != nil {
		t.Fatalf("Reader check failed while live: %v", err)
	}
	fixture.cancelReader()
	if err := stream.checkOperation(); err != nil {
		t.Fatalf("Stream qualification check merged the reader caller authority: %v", err)
	}
	readerErr := reader.checkOperation()
	if readerErr == nil || !errors.Is(readerErr, context.Canceled) {
		t.Fatalf("Reader check ignored the reader caller: %v", readerErr)
	}
	n, err := reader.Read(make([]byte, 4))
	if n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatal("Reader borrow lost the original caller cancellation", n, err)
	}
}
