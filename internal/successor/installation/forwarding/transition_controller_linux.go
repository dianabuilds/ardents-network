package forwarding

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"time"
)

type transitionOrigin struct {
	Process       transitionProcess `json:"process"`
	ProgramDevice uint64            `json:"program_device"`
	ProgramInode  uint64            `json:"program_inode"`
	ProgramSHA256 string            `json:"program_sha256"`
}

func transitionArguments(args []string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	operation := transitionOperation(args[0])
	wanted := 0
	switch operation {
	case "initial", "replacement":
		wanted = 7
	case "start":
		wanted = 6
	case "retirement":
		wanted = 3
	}
	return operation, wanted != 0 && len(args) == wanted
}

// RunTransition is only for the dedicated canonical Forwarding controller,
// with no other children. Its own executable and closed action are retained
// from birth through real scope join. A true result means only that its separate
// original-cut proof completed; it never means installation or recovery did.
func RunTransition(ctx context.Context, args []string, stopAfter uint32) (cutCompleted bool, returned error) {
	operation, admitted := transitionArguments(args)
	if ctx == nil || !admitted || os.Geteuid() != 0 || os.Getegid() != 0 || os.Getenv("ARDENTS_FORWARDING_TRANSITION_CHILD") != "" || stopAfter > transitionRecordLimit(operation) {
		return false, ErrInput
	}
	deadline, bounded := ctx.Deadline()
	if !bounded || !deadline.After(time.Now()) || deadline.After(time.Now().Add(2*time.Minute)) {
		return false, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	program, err := os.Open("/proc/self/exe")
	if err != nil {
		return false, err
	}
	programClosed := false
	defer func() {
		if !programClosed {
			returned = errors.Join(returned, program.Close())
		}
	}()
	info, err := program.Stat()
	if err != nil || !nodeAccess(info, info.Mode().Perm(), 0, 0, false) || info.Mode().Perm()&0022 != 0 {
		return false, errors.Join(ErrBinding, err)
	}
	channel, endpoint, err := transitionSocketPair()
	if err != nil {
		return false, err
	}
	channelClosed := false
	defer func() {
		if !channelClosed {
			returned = errors.Join(returned, channel.Close())
		}
	}()
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return false, errors.Join(err, endpoint.Close())
	}
	nonce := hex.EncodeToString(random[:])
	var environment []string
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "ARDENTS_FORWARDING_TRANSITION_") {
			environment = append(environment, item)
		}
	}
	environment = append(environment, "ARDENTS_FORWARDING_TRANSITION_CHILD=coordinator-v1", "ARDENTS_FORWARDING_TRANSITION_FD=4", "ARDENTS_FORWARDING_TRANSITION_NONCE="+nonce)
	argv := append([]string{os.Args[0], "forwarding"}, args...)
	if err := ctx.Err(); err != nil {
		return false, errors.Join(err, endpoint.Close())
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	scope, err := forkTransition(program, endpoint, argv, environment)
	endpointErr := endpoint.Close()
	if err != nil {
		return false, errors.Join(err, endpointErr)
	}
	defer func() { returned = errors.Join(returned, scope.join()) }()
	if endpointErr != nil {
		return false, endpointErr
	}
	origin, err := captureTransitionOrigin(program, scope)
	if err != nil {
		return false, err
	}
	cut, operationErr := controlTransition(ctx, scope, channel, nonce, operation, deadline, stopAfter)
	if cut == nil {
		return false, operationErr
	}
	// Every original controller callback, process/pidfd, executable and channel
	// has joined before the independent guarded historical proof is published.
	cleanup := errors.Join(program.Close(), channel.Close())
	programClosed, channelClosed = true, true
	if cleanup != nil {
		return false, errors.Join(operationErr, cleanup, cut.close())
	}
	completion, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	publication := publishTransitionCut(completion, cut, scope, origin)
	return publication == nil, errors.Join(operationErr, publication, ctx.Err())
}

func captureTransitionOrigin(program *os.File, scope *transitionScope) (transitionOrigin, error) {
	if program == nil || scope == nil || scope.birth.PID <= 1 || scope.birth.Started == 0 {
		return transitionOrigin{}, ErrInput
	}
	before, err := program.Stat()
	if err != nil || before.Size() <= 0 || before.Size() > 256<<20 || !nodeAccess(before, before.Mode().Perm(), 0, 0, false) || before.Mode().Perm()&0022 != 0 {
		return transitionOrigin{}, errors.Join(ErrBinding, err)
	}
	if _, err := program.Seek(0, io.SeekStart); err != nil {
		return transitionOrigin{}, err
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(program, before.Size()+1))
	after, statErr := program.Stat()
	if err != nil || statErr != nil || n != before.Size() || !sameInputFile(before, after) {
		return transitionOrigin{}, errors.Join(ErrBinding, err, statErr)
	}
	native, ok := infoSys(before)
	if !ok {
		return transitionOrigin{}, ErrBinding
	}
	return transitionOrigin{scope.birth, uint64(native.Dev), native.Ino, hex.EncodeToString(hash.Sum(nil))}, nil
}

func controlTransition(ctx context.Context, scope *transitionScope, channel *net.UnixConn, nonce, operation string, deadline time.Time, stopAfter uint32) (cut *transitionPrefix, returned error) {
	control := &transitionControl{scope: scope, channel: channel}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); control.deny() })
	defer func() {
		if !stop() {
			<-interrupted
		}
		if ctx.Err() != nil {
			control.deny()
		}
		var cleanup error
		cut, cleanup = control.finish()
		returned = errors.Join(returned, cleanup, ctx.Err())
	}()
	if err := channel.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return nil, err
	}
	if err := writeTransitionMessage(channel, transitionMessage{Schema: "ardents-forwarding-transition-hello-v1", Nonce: nonce, Operation: operation, Deadline: deadline.UnixNano()}); err != nil {
		return nil, err
	}
	ready, err := readTransitionMessage(channel, int32(scope.pid))
	if err != nil {
		return nil, err
	}
	if !sameTransitionMessage(ready, transitionMessage{Schema: "ardents-forwarding-transition-ready-v1", Nonce: nonce, Operation: operation}) {
		return nil, ErrBinding
	}
	if err := channel.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	var previous transitionMessage
	for sequence := uint32(1); ; sequence++ {
		message, err := readTransitionMessage(channel, int32(scope.pid))
		if err != nil {
			// Only the exact empty orderly-closure leaf is special. A wrapped or
			// joined EOF retains every independent native/cancellation failure.
			if err != io.EOF {
				return nil, err
			}
			if err := scope.awaitExit(ctx); err != nil {
				return nil, err
			}
			if err := scope.join(); err != nil {
				return nil, err
			}
			if !scope.status.Exited() || scope.status.ExitStatus() != 0 || scope.reaped != 1 || stopAfter != 0 {
				return nil, ErrRepairRequired
			}
			return nil, nil
		}
		if sequence == 0 || !transitionBoundarySchema(message) || message.Nonce != nonce || message.Operation != operation || message.Sequence != sequence || message.Deadline != 0 || message.Records != sequence || message.Records > transitionRecordLimit(operation) || message.Manager == "" || len(message.Manager) > 288 || len(message.NativeContext) == 0 || len(message.NativeContext) > 64<<10 {
			return nil, ErrBinding
		}
		if sequence > 1 && (message.Journal != previous.Journal || message.Device != previous.Device || message.Inode != previous.Inode || message.Generation != previous.Generation || message.BindingSHA256 != previous.BindingSHA256 || message.Manager != previous.Manager || !sameTransitionSnapshots(message, previous)) {
			return nil, ErrBinding
		}
		if err := scope.waitStopped(ctx); err != nil {
			return nil, err
		}
		prefix, err := openTransitionPrefix(ctx, message)
		if err != nil {
			return nil, err
		}
		if err := control.freeze(ctx, prefix); err != nil {
			return nil, errors.Join(err, prefix.close())
		}
		if scope.frozen != nil {
			scope.frozen()
		}
		if sequence == stopAfter {
			control.deny()
			return nil, nil
		}
		prefix, err = control.acknowledge(ctx)
		if err != nil {
			return nil, err
		}
		if err := prefix.close(); err != nil {
			return nil, err
		}
		if scope.acknowledged != nil {
			scope.acknowledged()
		}
		if err := writeTransitionMessage(channel, transitionMessage{Schema: "ardents-forwarding-transition-ack-v1", Nonce: nonce, Sequence: sequence, Operation: operation}); err != nil {
			return nil, err
		}
		if err := scope.resume(); err != nil {
			return nil, err
		}
		previous = message
	}
}

func sameTransitionSnapshots(a, b transitionMessage) bool {
	return a.Schema == b.Schema && string(a.StartIntent) == string(b.StartIntent) && string(a.Binding) == string(b.Binding) && string(a.OriginalContext) == string(b.OriginalContext)
}
