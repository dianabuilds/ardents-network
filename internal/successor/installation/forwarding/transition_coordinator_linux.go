package forwarding

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"syscall"
	"time"
)

type transitionContextKey struct{}

// The original controller owns cancellation and the deadline. In particular,
// no child timer can start cleanup while a reported record boundary is stopped.
type transitionDeadline struct {
	context.Context
	deadline time.Time
}

func (c transitionDeadline) Deadline() (time.Time, bool) { return c.deadline, true }

// TransitionCoordinator owns only the inherited original root-peer channel.
// It supplies no Release, domain join or recovery permission. Command composition
// serializes its use and joins Close before publishing any transition completion.
type TransitionCoordinator struct {
	ctx              context.Context
	channel          *net.UnixConn
	parent           int32
	nonce, operation string
	sequence         uint32
	boundary         transitionMessage
	closed           bool
	refreshed        bool
	terminal         error
}

func transitionOperation(action string) string {
	switch action {
	case "install-initial-stopped":
		return "initial"
	case "replace":
		return "replacement"
	case "start-initial", "start-selected", "restart":
		return "start"
	case "stop":
		return "retirement"
	}
	return ""
}

// OpenTransitionCoordinator selects only an explicitly inherited canonical
// child. A present malformed marker always refuses, including on read-only or
// non-root runtime commands. Absence preserves ordinary independent admission.
func OpenTransitionCoordinator(action string) (result *TransitionCoordinator, selected bool, returned error) {
	mode := os.Getenv("ARDENTS_FORWARDING_TRANSITION_CHILD")
	if mode == "" {
		return nil, false, nil
	}
	operation := transitionOperation(action)
	if mode != "coordinator-v1" || operation == "" || os.Geteuid() != 0 || os.Getegid() != 0 || os.Getenv("ARDENTS_FORWARDING_TRANSITION_FD") != "4" || !validStartDigest(os.Getenv("ARDENTS_FORWARDING_TRANSITION_NONCE")) {
		return nil, true, ErrInput
	}
	c := &TransitionCoordinator{parent: int32(os.Getppid()), nonce: os.Getenv("ARDENTS_FORWARDING_TRANSITION_NONCE"), operation: operation}
	defer func() {
		if returned != nil {
			returned = errors.Join(returned, c.Close())
			result = nil
		}
	}()
	if c.parent <= 1 {
		return nil, true, ErrInput
	}
	program := os.NewFile(3, "original Forwarding transition program")
	self, err := os.Open("/proc/self/exe")
	if err != nil {
		return nil, true, errors.Join(err, program.Close())
	}
	info, statErr := program.Stat()
	selfInfo, selfErr := self.Stat()
	closeErr := errors.Join(program.Close(), self.Close())
	if statErr != nil || selfErr != nil || closeErr != nil || !os.SameFile(info, selfInfo) || !nodeAccess(info, info.Mode().Perm(), 0, 0, false) || info.Mode().Perm()&0022 != 0 {
		return nil, true, errors.Join(ErrBinding, statErr, selfErr, closeErr)
	}
	raw := os.NewFile(4, "original Forwarding transition endpoint")
	kind, kindErr := syscall.GetsockoptInt(int(raw.Fd()), syscall.SOL_SOCKET, syscall.SO_TYPE)
	passcred, credErr := syscall.GetsockoptInt(int(raw.Fd()), syscall.SOL_SOCKET, syscall.SO_PASSCRED)
	if kindErr != nil || credErr != nil || kind != syscall.SOCK_SEQPACKET || passcred != 1 {
		return nil, true, errors.Join(ErrBinding, kindErr, credErr, raw.Close())
	}
	connection, err := net.FileConn(raw)
	closeErr = raw.Close()
	if err != nil {
		return nil, true, errors.Join(err, closeErr)
	}
	var ok bool
	c.channel, ok = connection.(*net.UnixConn)
	if !ok || closeErr != nil {
		return nil, true, errors.Join(ErrBinding, closeErr, connection.Close())
	}
	if err := c.channel.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return nil, true, err
	}
	hello, err := readTransitionMessage(c.channel, c.parent)
	if err != nil {
		return nil, true, err
	}
	deadline := time.Unix(0, hello.Deadline)
	wanted := transitionMessage{Schema: "ardents-forwarding-transition-hello-v1", Nonce: c.nonce, Operation: operation, Deadline: hello.Deadline}
	if !sameTransitionMessage(hello, wanted) || !deadline.After(time.Now()) || deadline.After(time.Now().Add(2*time.Minute)) {
		return nil, true, ErrBinding
	}
	parentProgram, err := os.Open("/proc/" + strconv.Itoa(int(c.parent)) + "/exe")
	if err != nil {
		return nil, true, err
	}
	parentInfo, err := parentProgram.Stat()
	closeErr = parentProgram.Close()
	if err != nil || closeErr != nil || !os.SameFile(selfInfo, parentInfo) || int32(os.Getppid()) != c.parent {
		return nil, true, errors.Join(ErrBinding, err, closeErr)
	}
	if err := writeTransitionMessage(c.channel, transitionMessage{Schema: "ardents-forwarding-transition-ready-v1", Nonce: c.nonce, Operation: operation}); err != nil {
		return nil, true, err
	}
	if err := c.channel.SetDeadline(time.Time{}); err != nil {
		return nil, true, err
	}
	c.ctx = context.WithValue(transitionDeadline{Context: context.Background(), deadline: deadline}, transitionContextKey{}, c)
	return c, true, nil
}

func sameTransitionMessage(a, b transitionMessage) bool {
	x, xe := encodeTransitionMessage(a)
	y, ye := encodeTransitionMessage(b)
	return xe == nil && ye == nil && bytes.Equal(x, y)
}

func (c *TransitionCoordinator) Context() context.Context {
	if c == nil {
		return nil
	}
	return c.ctx
}

// TransitionContext preserves the sole parent-owned timer in a managed child.
// Ordinary commands still receive their own finite operation deadline.
func TransitionContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		return nil, func() {}
	}
	if parent != nil && parent.Value(transitionContextKey{}) != nil {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}

// bindTransition is called only by the actual admitted operation, with its own
// immutable original binding and independently captured concrete native context.
// It does not replace any of that operation's checks or native custody.
func bindTransition(ctx context.Context, operation, journal, generation string, binding, nativeContext []byte, manager string) error {
	c, _ := ctx.Value(transitionContextKey{}).(*TransitionCoordinator)
	if c == nil {
		return nil
	}
	if c.closed || c.terminal != nil || c.sequence != 0 || c.boundary.Journal != "" || c.operation != operation || !validPath(journal) || !validStartDigest(generation) || len(binding) == 0 || len(binding) > 32<<10 || len(nativeContext) == 0 || len(nativeContext) > 64<<10 || manager == "" || len(manager) > 288 {
		return ErrBinding
	}
	c.boundary = transitionMessage{Schema: "ardents-forwarding-transition-boundary-v1", Nonce: c.nonce, Operation: operation, Journal: journal, Generation: generation, BindingSHA256: digest(binding), Binding: bytes.Clone(binding), Manager: manager, OriginalContext: bytes.Clone(nativeContext), NativeContext: bytes.Clone(nativeContext)}
	return nil
}

// Only the actual root Start supplies its already computed original intent.
// The same immutable bytes are later written by its barrier, with no replay.
func bindStartTransition(ctx context.Context, journal, generation string, binding, nativeContext []byte, manager string, intent []byte) error {
	message := transitionMessage{Schema: "ardents-forwarding-transition-boundary-v2", Operation: "start", Journal: journal, Generation: generation, BindingSHA256: digest(binding), Binding: binding, StartIntent: intent}
	if _, err := readTransitionStartIntent(message); err != nil {
		return err
	}
	if err := bindTransition(ctx, "start", journal, generation, binding, nativeContext, manager); err != nil {
		return err
	}
	c, _ := ctx.Value(transitionContextKey{}).(*TransitionCoordinator)
	if c != nil {
		c.boundary.Schema = message.Schema
		c.boundary.StartIntent = bytes.Clone(intent)
	}
	return nil
}

// Called only after successful original Sync AND exact native readback. Between
// send and uncatchable self-stop there is no next effect, cleanup or timer.
func reportTransition(ctx context.Context, journal *initialFile, count uint32) (returned error) {
	c, _ := ctx.Value(transitionContextKey{}).(*TransitionCoordinator)
	if c == nil {
		return nil
	}
	defer func() { c.terminal = errors.Join(c.terminal, returned) }()
	if c.closed || c.terminal != nil || journal == nil || c.boundary.Journal != journal.path || count != c.sequence+1 || c.sequence == ^uint32(0) || int32(os.Getppid()) != c.parent {
		return ErrBinding
	}
	if err := errors.Join(ctx.Err(), observeInitialFile(journal)); err != nil {
		return err
	}
	native, ok := infoSys(journal.info)
	if !ok || native.Dev == 0 || native.Ino == 0 {
		return ErrBinding
	}
	c.sequence++
	message := c.boundary
	message.Sequence, message.Records = c.sequence, count
	message.Device, message.Inode, message.PrefixSHA256 = uint64(native.Dev), native.Ino, digest(journal.body)
	if err := writeTransitionMessage(c.channel, message); err != nil {
		return err
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGSTOP); err != nil {
		return err
	}
	ack, err := readTransitionMessage(c.channel, c.parent)
	if err != nil {
		return err
	}
	if !sameTransitionMessage(ack, transitionMessage{Schema: "ardents-forwarding-transition-ack-v1", Nonce: c.nonce, Sequence: c.sequence, Operation: c.operation}) {
		return ErrBinding
	}
	return ctx.Err()
}

func finishTransition(ctx context.Context) error {
	c, _ := ctx.Value(transitionContextKey{}).(*TransitionCoordinator)
	return c.Close()
}

func (c *TransitionCoordinator) Close() error {
	if c == nil {
		return nil
	}
	if !c.closed {
		c.closed = true
		if c.channel != nil {
			c.terminal = errors.Join(c.terminal, c.channel.Close())
		}
		clear(c.boundary.Binding)
		clear(c.boundary.OriginalContext)
		clear(c.boundary.NativeContext)
		clear(c.boundary.StartIntent)
	}
	return c.terminal
}
