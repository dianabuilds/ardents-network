package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"testing"
	"time"
)

type forwardingPrivateDiagnosticError struct{ printed *bool }

func (e forwardingPrivateDiagnosticError) Error() string {
	*e.printed = true
	return "private diagnostic path and peer"
}

type forwardingCyclicDiagnosticError struct{}

func (forwardingCyclicDiagnosticError) Error() string   { return "private cycle" }
func (e forwardingCyclicDiagnosticError) Unwrap() error { return e }

func TestForwardingWorkRefusalOutputNeverPrintsPrivateCause(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	printed := false
	cause := forwardingPrivateDiagnosticError{&printed}
	if err := writeForwardingWorkRefusal(t.Context(), writer, nil, cause); err != nil {
		t.Fatal("bounded diagnostic write failed", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil || printed || bytes.Contains(raw, []byte("private")) || !bytes.Contains(raw, []byte(`"phase":"work-refused"`)) || !bytes.Contains(raw, []byte(`"kind":"other"`)) {
		t.Fatal("work refusal leaked its original private cause", string(raw), err)
	}
}

func TestForwardingErrorShapeNeverPrintsOrClassifiesPrivateCauses(t *testing.T) {
	printed := false
	private := forwardingPrivateDiagnosticError{&printed}
	wrapped := fmt.Errorf("private wrapper: %w", context.Canceled)
	cause := errors.Join(private, wrapped, io.ErrUnexpectedEOF, syscall.EPIPE)
	printed = false
	raw, err := json.Marshal(forwardingErrorShape(cause))
	if err != nil || printed || bytes.Contains(raw, []byte("private")) || !bytes.Contains(raw, []byte(`"kind":"wrapped"`)) || !bytes.Contains(raw, []byte(`"kind":"canceled"`)) || !bytes.Contains(raw, []byte(`"errno":32`)) {
		t.Fatal("safe shape lost categories or observed private text", err)
	}
	if forwardingCancellationOnly(cause) || forwardingCancellationOnly(wrapped) {
		t.Fatal("diagnostic changed physical-wrapper refusal")
	}
	cycle := forwardingErrorShape(forwardingCyclicDiagnosticError{})
	if len(cycle) != 65 || cycle[64].Kind != "truncated" {
		t.Fatal("cyclic diagnostic was not finitely bounded")
	}
}

// Local terminal grammar only. These values are never sent to Installation or
// used as a successful runtime/root report, native join or restart proof.
func TestForwardingExplicitStopMarkerCannotHideLatePhysicalFailure(t *testing.T) {
	marker := &forwardingExplicitStop{context.Canceled}
	if !completedExplicitForwardingStop(marker) {
		t.Fatal("exact completed marker grammar refused")
	}
	for _, err := range []error{
		context.Canceled, errors.New("forwarding explicitly stopped"),
		errors.Join(marker, errors.New("late physical Close failed")),
		fmt.Errorf("foreign wrapper: %w", marker),
		&forwardingExplicitStop{fmt.Errorf("physical Close: %w", context.Canceled)},
		&forwardingExplicitStop{errors.Join(context.Canceled, errors.New("State Close failed"))},
		&forwardingExplicitStop{context.DeadlineExceeded},
	} {
		if completedExplicitForwardingStop(err) {
			t.Fatal("foreign/late result became clean process exit", err)
		}
	}
}

func TestForwardingRuntimeOutputDeadlineDoesNotWaitForPeerDrain(t *testing.T) {
	reader, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer output.Close()
	fd := int(output.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	var prefix []byte
	chunk := bytes.Repeat([]byte("p"), 4096)
	for {
		count, err := syscall.Write(fd, chunk)
		if count > 0 {
			prefix = append(prefix, chunk[:count]...)
		}
		if errors.Is(err, syscall.EAGAIN) {
			break
		}
		if err != nil || len(prefix) > 1<<20 {
			t.Fatal("fill original output", len(prefix), err)
		}
	}
	if err := syscall.SetNonblock(fd, false); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	joined := make(chan error, 1)
	go func() { joined <- writeForwardingRuntimeEvent(ctx, output, []byte("failed\n")) }()
	completed := false
	select {
	case err := <-joined:
		completed = true
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Error("saturated original output lost its caller deadline", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("saturated original output retained command cleanup beyond caller deadline")
	}
	// Even the red regression joins its original blocked writer before return.
	// Draining only the existing prefix permits a buggy blocking write to finish.
	drained := make([]byte, len(prefix))
	if _, err := io.ReadFull(reader, drained); err != nil || !bytes.Equal(drained, prefix) {
		t.Fatal("original output prefix changed", err)
	}
	if !completed {
		if err := <-joined; err != nil {
			t.Log("original blocked writer joined after peer drain", err)
		}
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	late, err := io.ReadAll(reader)
	if err != nil || len(late) != 0 {
		t.Error("expired original report wrote after deadline", string(late), err)
	}
}

func TestForwardingRuntimeOutputRequiresInterruptibleNativeDescriptor(t *testing.T) {
	var out bytes.Buffer
	if err := writeForwardingRuntimeEvent(t.Context(), &out, []byte("failed\n")); err == nil || out.Len() != 0 {
		t.Fatal("uninterruptible writer was used", err, out.Len())
	}
}

func TestForwardingRuntimeOutputCancellationAndBrokenPeer(t *testing.T) {
	t.Run("canceled-before-write", func(t *testing.T) {
		reader, output, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		defer output.Close()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := writeForwardingRuntimeEvent(ctx, output, []byte("failed\n")); err != context.Canceled {
			t.Fatal("original cancellation lost", err)
		}
		if err := output.Close(); err != nil {
			t.Fatal(err)
		}
		if raw, err := io.ReadAll(reader); err != nil || len(raw) != 0 {
			t.Fatal("canceled output wrote bytes", len(raw), err)
		}
	})
	t.Run("broken-peer", func(t *testing.T) {
		reader, output, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer output.Close()
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
		if err := writeForwardingRuntimeEvent(t.Context(), output, []byte("failed\n")); !errors.Is(err, syscall.EPIPE) {
			t.Fatal("original peer write failure lost", err)
		}
	})
}

func TestForwardingCancellationClassificationRetainsPhysicalWrappers(t *testing.T) {
	if !forwardingCancellationOnly(errors.Join(context.Canceled, context.Canceled)) {
		t.Fatal("ordinary cancellation join refused")
	}
	for _, err := range []error{fmt.Errorf("native: %w", context.Canceled), errors.Join(context.Canceled, fmt.Errorf("native: %w", context.Canceled)), context.DeadlineExceeded} {
		if forwardingCancellationOnly(err) {
			t.Fatal("physical/late wrapper erased", err)
		}
	}
}
