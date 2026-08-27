package portable

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	attachmentName           = "endpoint.sock"
	maximumAttachmentPathLen = 96
)

// State is one visible H4-1A Portable Endpoint lifecycle state.
type State string

const (
	StateStarting     State = "starting"
	StateReady        State = "ready"
	StateBlocked      State = "blocked"
	StateStale        State = "stale"
	StateIncompatible State = "incompatible"
	StateStopped      State = "stopped"
)

// Reason is a stable, non-secret classification for a lifecycle event.
type Reason string

const (
	ReasonOwnerBusy              Reason = "owner-busy"
	ReasonLockError              Reason = "lock-error"
	ReasonUnexpectedRuntimeEntry Reason = "unexpected-runtime-entry"
	ReasonStaleRecoveryFailed    Reason = "stale-recovery-failed"
	ReasonAttachmentBindFailed   Reason = "attachment-bind-failed"
	ReasonLocalProfileInvalid    Reason = "local-profile-invalid"
)

// Event is one visible Portable Endpoint lifecycle observation.
type Event struct {
	State      State
	Reason     Reason
	Attachment string
}

// Error carries a stable failure classification without making paths or
// operating-system details part of the caller contract.
type Error struct {
	Reason Reason
	cause  error
}

func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	if err.cause == nil {
		return string(err.Reason)
	}
	return fmt.Sprintf("%s: %v", err.Reason, err.cause)
}

func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

// Config contains final, platform-resolved, per-user roots. Program bytes are
// deliberately not represented: they are outside the Portable state profile.
type Config struct {
	ConfigHome  string
	StateHome   string
	CacheHome   string
	RuntimeHome string
}

// Runtime owns one live Portable Endpoint profile until Close returns. It
// exposes only a generic local attachment, not an application capability.
type Runtime struct {
	attachment string
	listener   *net.UnixListener
	lease      ownerLease
	work       sync.WaitGroup
	closeOnce  sync.Once
	closeErr   error
}

// Open claims the configured local profile, recovers only a stale filesystem
// socket after the claim succeeds, binds a fresh attachment, and proves that
// the attachment answers before returning readiness.
func Open(config Config) (*Runtime, error) {
	paths, err := prepareRoots(config)
	if err != nil {
		return nil, lifecycleError(ReasonLocalProfileInvalid, err)
	}
	lease, err := acquireOwnerLease(paths.lock)
	if err != nil {
		return nil, err
	}
	attachment := filepath.Join(paths.runtime, attachmentName)
	if len([]byte(attachment)) > maximumAttachmentPathLen {
		return nil, errors.Join(lifecycleError(ReasonLocalProfileInvalid, errors.New("local attachment path exceeds its declared budget")), lease.release())
	}
	if err := recoverAttachment(attachment); err != nil {
		return nil, errors.Join(err, lease.release())
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: attachment, Net: "unix"})
	if err != nil {
		return nil, errors.Join(lifecycleError(ReasonAttachmentBindFailed, err), lease.release())
	}
	listener.SetUnlinkOnClose(false)
	if err := secureAttachment(attachment); err != nil {
		_ = listener.Close()
		return nil, errors.Join(lifecycleError(ReasonAttachmentBindFailed, err), removeExpectedAttachment(attachment), lease.release())
	}
	runtime := &Runtime{attachment: attachment, listener: listener, lease: lease}
	runtime.work.Add(1)
	go runtime.serveAttachment()
	if err := probeAttachment(attachment); err != nil {
		return nil, errors.Join(lifecycleError(ReasonAttachmentBindFailed, err), runtime.Close())
	}
	return runtime, nil
}

// Attachment returns the exact local Unix-socket path owned while Runtime is
// live. It is not an ambient TCP address and carries no administration grant.
func (runtime *Runtime) Attachment() string {
	if runtime == nil {
		return ""
	}
	return runtime.attachment
}

// Close stops attachment admission, removes only its expected socket entry,
// and releases the held process-lifetime owner lock.
func (runtime *Runtime) Close() error {
	if runtime == nil {
		return nil
	}
	runtime.closeOnce.Do(func() {
		closeErr := runtime.listener.Close()
		runtime.work.Wait()
		runtime.closeErr = errors.Join(closeErr, removeExpectedAttachment(runtime.attachment), runtime.lease.release())
	})
	return runtime.closeErr
}

// Wait holds the claimed Portable profile until the caller requests a normal
// stop, then performs its terminal cleanup. It lets a higher composition owner
// establish release or network prerequisites before it emits readiness.
func (runtime *Runtime) Wait(ctx context.Context) error {
	if runtime == nil || ctx == nil {
		return errors.New("portable runtime is unavailable")
	}
	<-ctx.Done()
	return runtime.Close()
}

// Run owns the normal foreground Portable lifecycle. A requested context stop
// is a clean participant stop and therefore returns nil after cleanup.
func Run(ctx context.Context, config Config, observe func(Event)) error {
	emit(observe, Event{State: StateStarting})
	runtime, err := Open(config)
	if err != nil {
		emit(observe, FailureEvent(err))
		return err
	}
	emit(observe, Event{State: StateReady, Attachment: runtime.Attachment()})
	err = runtime.Wait(ctx)
	if err != nil {
		emit(observe, Event{State: StateBlocked, Reason: ReasonLockError})
		return err
	}
	emit(observe, Event{State: StateStopped})
	return nil
}

func (runtime *Runtime) serveAttachment() {
	defer runtime.work.Done()
	for {
		connection, err := runtime.listener.AcceptUnix()
		if err != nil {
			return
		}
		runtime.work.Add(1)
		go func() {
			defer runtime.work.Done()
			defer connection.Close()
			_ = connection.SetDeadline(time.Now().Add(time.Second))
			request := make([]byte, len("probe\n"))
			if _, err := io.ReadFull(connection, request); err != nil || string(request) != "probe\n" {
				return
			}
			_, _ = io.WriteString(connection, "ready\n")
		}()
	}
}

func probeAttachment(path string) error {
	connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(time.Second)); err != nil {
		return err
	}
	if _, err := io.WriteString(connection, "probe\n"); err != nil {
		return err
	}
	response, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil {
		return err
	}
	if response != "ready\n" {
		return errors.New("unexpected attachment probe response")
	}
	return nil
}

func recoverAttachment(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return lifecycleError(ReasonStaleRecoveryFailed, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return lifecycleError(ReasonUnexpectedRuntimeEntry, errors.New("attachment is not a socket"))
	}
	if err := os.Remove(path); err != nil {
		return lifecycleError(ReasonStaleRecoveryFailed, err)
	}
	return nil
}

func removeExpectedAttachment(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return errors.New("attachment changed to an unexpected entry")
	}
	return os.Remove(path)
}

func lifecycleError(reason Reason, cause error) *Error { return &Error{Reason: reason, cause: cause} }

// FailureEvent classifies a Portable startup or shutdown error into the
// stable lifecycle observation that a higher Endpoint composition can expose.
// It intentionally does not reveal a filesystem path or operating-system
// detail.
func FailureEvent(err error) Event {
	var lifecycle *Error
	if errors.As(err, &lifecycle) {
		switch lifecycle.Reason {
		case ReasonStaleRecoveryFailed, ReasonUnexpectedRuntimeEntry:
			return Event{State: StateStale, Reason: lifecycle.Reason}
		case ReasonOwnerBusy, ReasonLockError, ReasonAttachmentBindFailed:
			return Event{State: StateBlocked, Reason: lifecycle.Reason}
		default:
			return Event{State: StateIncompatible, Reason: lifecycle.Reason}
		}
	}
	return Event{State: StateIncompatible, Reason: ReasonLocalProfileInvalid}
}

func emit(observe func(Event), event Event) {
	if observe != nil {
		observe(event)
	}
}
