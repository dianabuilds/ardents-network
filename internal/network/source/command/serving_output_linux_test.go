//go:build linux

package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source/serving"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestSourceNativeOutputLossAndBackpressureAreFinite(t *testing.T) {
	for _, fault := range []string{"peer-close", "backpressure"} {
		t.Run(fault, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			defer writer.Close()
			if fault == "peer-close" {
				if err := reader.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				fd := int(writer.Fd())
				if err := syscall.SetNonblock(fd, true); err != nil {
					t.Fatal(err)
				}
				for {
					_, err := syscall.Write(fd, make([]byte, 4096))
					if errors.Is(err, syscall.EAGAIN) {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			started := time.Now()
			err = Events(writer, io.Discard)(t.Context(), serving.Event{Phase: "listening", Outcome: "original-source"})
			if fault == "peer-close" && !errors.Is(err, syscall.EPIPE) {
				t.Fatal("original output cause lost", err)
			}
			if fault == "backpressure" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("blocked output did not refuse", err)
			}
			if time.Since(started) > 2*time.Second {
				t.Fatal("output exceeded its finite bound")
			}
		})
	}
}

type sourcePipeLifetime struct{ closed bool }

func (*sourcePipeLifetime) Current() (state.Snapshot, error) { return state.Snapshot{}, nil }
func (*sourcePipeLifetime) Wait(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
func (owner *sourcePipeLifetime) Close() error { owner.closed = true; return nil }

type sourceNotice struct{ observed chan struct{} }

func (notice sourceNotice) Write(raw []byte) (int, error) {
	if bytes.Contains(raw, []byte("completion-unavailable")) {
		close(notice.observed)
	}
	return len(raw), nil
}

type failedSourceOutput struct{ err error }

func (f failedSourceOutput) Write([]byte) (int, error) { return 0, f.err }

func TestSourceCallerCancellationDuringBlockedReadyOutputRetainsCaller(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	fd := int(writer.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	for {
		_, err := syscall.Write(fd, make([]byte, 4096))
		if errors.Is(err, syscall.EAGAIN) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, deny := context.WithCancel(ctx)
	defer deny()
	checked := make(chan struct{})
	owner := &sourcePipeLifetime{}
	joined := make(chan error, 1)
	go func() {
		joined <- superviseSource(ctx, owner, func() error {
			select {
			case <-checked:
			default:
				close(checked)
			}
			return nil
		}, deny, writer, io.Discard, nil)
	}()
	<-checked
	cancel()
	if err := <-joined; err != nil || !owner.closed {
		t.Fatal("original caller cancellation became an output/background failure", err)
	}
}

type sourceFailureWriter func([]byte) (int, error)

func (writer sourceFailureWriter) Write(raw []byte) (int, error) { return writer(raw) }

type sourceLifetime interface {
	Current() (state.Snapshot, error)
	Wait(context.Context) error
	Close() error
}

func superviseSource(ctx context.Context, owner sourceLifetime, check func() error, deny context.CancelFunc, out, diagnostic io.Writer, joinInputs func() error) error {
	return serving.Supervise(ctx, owner, check, deny, Events(out, diagnostic), joinInputs)
}
