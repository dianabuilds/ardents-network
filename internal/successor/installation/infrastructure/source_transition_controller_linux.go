package infrastructure

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// RunSourceTransition owns the original canonical initial/replacement
// coordinator from native birth through physical join. It grants no recovery
// permission: an interrupted operation still requires its separate completed
// original-cut proof. It must run only in the dedicated canonical process,
// which has no other children and exits after this command returns.
func RunSourceTransition(ctx context.Context, args []string) (err error) {
	if ctx == nil || len(args) != 7 || os.Geteuid() != 0 || os.Getegid() != 0 || os.Getenv("ARDENTS_SOURCE_TRANSITION_CHILD") != "" {
		return ErrInput
	}
	operation := ""
	switch args[0] {
	case "install-initial-stopped":
		operation = "initial"
	case "replace-source":
		operation = "replacement"
	default:
		return ErrInput
	}
	deadline, ok := ctx.Deadline()
	if !ok || !deadline.After(time.Now()) || deadline.After(time.Now().Add(2*time.Minute)) {
		return ErrInput
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	program, err := os.Open("/proc/self/exe")
	if err != nil {
		return err
	}
	programClosed := false
	defer func() {
		if !programClosed {
			err = errors.Join(err, program.Close())
		}
	}()
	info, err := program.Stat()
	if err != nil || !sourceAccess(info, info.Mode().Perm(), 0, 0, false) || info.Mode().Perm()&0022 != 0 {
		return errors.Join(ErrBinding, err)
	}
	channel, endpoint, err := sourceTransitionSocketPair()
	if err != nil {
		return err
	}
	channelClosed := false
	defer func() {
		if !channelClosed {
			err = errors.Join(err, channel.Close())
		}
	}()
	var random [32]byte
	if _, err = rand.Read(random[:]); err != nil {
		return errors.Join(err, endpoint.Close())
	}
	nonce := hex.EncodeToString(random[:])
	environment := []string{}
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "ARDENTS_SOURCE_TRANSITION_") {
			environment = append(environment, value)
		}
	}
	environment = append(environment, "ARDENTS_SOURCE_TRANSITION_CHILD=coordinator-v1", "ARDENTS_SOURCE_TRANSITION_FD=4", "ARDENTS_SOURCE_TRANSITION_NONCE="+nonce)
	argv := append([]string{os.Args[0], "installation"}, args...)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	scope, err := forkSourceTransition(program, endpoint, argv, environment)
	endpointClosed := endpoint.Close()
	if err != nil {
		return errors.Join(err, endpointClosed)
	}
	defer func() { err = errors.Join(err, scope.join()) }()
	if endpointClosed != nil {
		return endpointClosed
	}
	origin, err := captureSourceTransitionOrigin(program, scope)
	if err != nil {
		return err
	}
	cut, err := controlSourceTransition(ctx, scope, channel, nonce, operation, deadline)
	if cut == nil {
		return err
	}
	// controlSourceTransition has joined its callback and native scope. Close
	// these remaining actual observers before any accepting proof publication.
	cleanup := errors.Join(program.Close(), channel.Close())
	programClosed, channelClosed = true, true
	if cleanup != nil {
		return errors.Join(err, cleanup, cut.close())
	}
	completion, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	return errors.Join(err, publishSourceCutResult(completion, cut, scope, origin))
}

// The same physical protocol is used by the canonical launcher and native
// file/process controls. Tests cannot replace a successful packet, stop, inode
// read or wait result with a fixture observation.
func controlSourceTransition(ctx context.Context, scope *sourceTransitionScope, channel *net.UnixConn, nonce, operation string, deadline time.Time) (cut *sourceTransitionCut, err error) {
	control := &sourceTransitionControl{scope: scope, channel: channel}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); control.deny() })
	defer func() {
		if !stop() {
			<-interrupted
		}
		var cleanup error
		cut, cleanup = control.finish(ctx)
		err = errors.Join(err, cleanup, ctx.Err())
	}()
	if err = channel.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return nil, err
	}
	hello := sourceTransitionMessage{Schema: "ardents-source-transition-hello-v1", Nonce: nonce, Operation: operation, Deadline: deadline.UnixNano()}
	if err = writeSourceTransitionMessage(channel, hello); err != nil {
		return nil, err
	}
	ready, err := readSourceTransitionMessage(channel, int32(scope.pid))
	if err != nil {
		return nil, err
	}
	if ready != (sourceTransitionMessage{Schema: "ardents-source-transition-ready-v1", Nonce: nonce, Operation: operation}) {
		return nil, ErrBinding
	}
	if err = channel.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	sequence := uint32(0)
	var previous sourceTransitionMessage
	for {
		message, readErr := readSourceTransitionMessage(channel, int32(scope.pid))
		if readErr != nil {
			// An orderly channel EOF can precede the final kernel exit. Retain
			// the original leader without reaping until its scope is denied.
			if !errors.Is(readErr, io.EOF) {
				return nil, readErr
			}
			if err = scope.awaitExit(ctx); err != nil {
				return nil, err
			}
			if err = scope.join(); err != nil {
				return nil, err
			}
			if !scope.status.Exited() || scope.status.ExitStatus() != 0 || scope.reaped != 1 {
				// Successful ordinary completion must have joined every helper in
				// the coordinator. Killing an adopted helper during scope cleanup
				// is physical release, never that helper's domain completion.
				return nil, fmt.Errorf("original Source transition exit %d, reaped processes %d: %w", uint32(scope.status), scope.reaped, ErrSourceRepairRequired)
			}
			return nil, nil
		}
		if sequence == ^uint32(0) {
			return nil, ErrBinding
		}
		sequence++
		if message.Schema != "ardents-source-transition-boundary-v1" || message.Nonce != nonce || message.Operation != operation || message.Sequence != sequence || message.Deadline != 0 || !sourcePath(message.Journal) || !sourceInitialSHA(message.Generation) || !sourceInitialSHA(message.BindingSHA256) || !sourceInitialSHA(message.PrefixSHA256) || message.Device == 0 || message.Inode == 0 || message.Records != int(sequence)+1 || message.Records > 115 {
			return nil, ErrBinding
		}
		if len(message.ManagerIdentity) > 288 {
			return nil, ErrBinding
		}
		if sequence > 1 && (message.Journal != previous.Journal || message.Device != previous.Device || message.Inode != previous.Inode || message.Generation != previous.Generation || message.BindingSHA256 != previous.BindingSHA256 || message.ManagerIdentity != previous.ManagerIdentity || message.Records != previous.Records+1) {
			return nil, ErrBinding
		}
		if err = scope.waitStopped(ctx); err != nil {
			return nil, err
		}
		frozen, e := observeSourceTransitionCut(ctx, message)
		if e != nil {
			return nil, e
		}
		if err = control.freeze(ctx, frozen); err != nil {
			return nil, errors.Join(err, frozen.close())
		}
		if scope.frozen != nil {
			scope.frozen()
		}
		frozen, err = control.acknowledge(ctx)
		if err != nil {
			return nil, err
		}
		if err = frozen.close(); err != nil {
			return nil, err
		}
		// From BEFORE this send the next effect is outstanding. Even a failed
		// or partial ACK can never supply a completed original-cut result.
		if err = writeSourceTransitionMessage(channel, sourceTransitionMessage{Schema: "ardents-source-transition-ack-v1", Nonce: nonce, Sequence: sequence, Operation: operation}); err != nil {
			return nil, err
		}
		if err = scope.resume(); err != nil {
			return nil, err
		}
		previous = message
	}
}

func sourceTransitionSocketPair() (channel *net.UnixConn, endpoint *os.File, err error) {
	pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_SEQPACKET|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	first := os.NewFile(uintptr(pair[0]), "source-transition-controller-channel")
	second := os.NewFile(uintptr(pair[1]), "source-transition-coordinator-channel")
	for _, fd := range pair {
		if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_PASSCRED, 1); err != nil {
			return nil, nil, errors.Join(err, first.Close(), second.Close())
		}
	}
	connection, err := net.FileConn(first)
	closed := first.Close()
	if err != nil {
		return nil, nil, errors.Join(err, closed, second.Close())
	}
	channel, ok := connection.(*net.UnixConn)
	if !ok || closed != nil {
		return nil, nil, errors.Join(ErrBinding, closed, connection.Close(), second.Close())
	}
	return channel, second, nil
}
