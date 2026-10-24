package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"syscall"
	"time"
)

const sourceTransitionPacketLimit = 4096

// One fixed private channel grammar, not an Authorization or public receipt.
// Every field is present and messages are canonical compact JSON plus LF.
type sourceTransitionMessage struct {
	Schema          string `json:"schema"`
	Nonce           string `json:"nonce"`
	Sequence        uint32 `json:"sequence"`
	Operation       string `json:"operation"`
	Deadline        int64  `json:"deadline"`
	Journal         string `json:"journal"`
	Device          uint64 `json:"device"`
	Inode           uint64 `json:"inode"`
	PrefixSHA256    string `json:"prefix_sha256"`
	Records         int    `json:"records"`
	Generation      string `json:"generation"`
	BindingSHA256   string `json:"binding_sha256"`
	ManagerIdentity string `json:"manager_identity"`
}

type sourceTransitionContextKey struct{}

// The actual original controller owns the single deadline/cancellation timer.
// It denies ACK before interruption. This managed child has no competing
// deadline callback that could start Close while a reported boundary freezes.
// Inner native call timeouts still use the same declared original deadline.
type sourceTransitionDeadline struct {
	context.Context
	deadline time.Time
}

func (c sourceTransitionDeadline) Deadline() (time.Time, bool) { return c.deadline, true }

type SourceTransitionCoordinator struct {
	ctx              context.Context
	cancel           context.CancelFunc
	channel          *net.UnixConn
	parent           int32
	nonce, operation string
	sequence         uint32
	closed           bool
	terminal         error
}

// OpenSourceTransitionCoordinator joins only the inherited exact-program
// private child channel. A missing marker selects ordinary command composition;
// a malformed/present marker is an error, never a fallback. Channel/progress
// facts grant no Enrollment, Release, State or Installation permission.
func OpenSourceTransitionCoordinator(operation string) (result *SourceTransitionCoordinator, selected bool, err error) {
	mode := os.Getenv("ARDENTS_SOURCE_TRANSITION_CHILD")
	if mode == "" {
		return nil, false, nil
	}
	selected = true
	if mode != "coordinator-v1" || os.Geteuid() != 0 || os.Getegid() != 0 || (operation != "initial" && operation != "replacement") || os.Getenv("ARDENTS_SOURCE_TRANSITION_FD") != "4" || !sourceInitialSHA(os.Getenv("ARDENTS_SOURCE_TRANSITION_NONCE")) {
		return nil, true, ErrInput
	}
	c := &SourceTransitionCoordinator{nonce: os.Getenv("ARDENTS_SOURCE_TRANSITION_NONCE"), operation: operation, parent: int32(os.Getppid())}
	defer func() {
		if err != nil {
			err = errors.Join(err, c.Close())
			result = nil
		}
	}()
	if c.parent <= 1 {
		return nil, true, ErrInput
	}
	// Descriptor3 is the exact executable retained by the original launcher;
	// descriptor4 is its private packet endpoint. Neither is caller-configurable.
	program := os.NewFile(3, "source-transition-original-program")
	if program == nil {
		return nil, true, ErrInput
	}
	programInfo, pe := program.Stat()
	self, se := os.Open("/proc/self/exe")
	if se != nil {
		return nil, true, errors.Join(se, pe, program.Close())
	}
	selfInfo, se := self.Stat()
	closed := errors.Join(program.Close(), self.Close())
	if pe != nil || se != nil || closed != nil || !os.SameFile(programInfo, selfInfo) || !sourceAccess(programInfo, programInfo.Mode().Perm(), 0, 0, false) || programInfo.Mode().Perm()&0022 != 0 {
		return nil, true, errors.Join(ErrBinding, pe, se, closed)
	}
	raw := os.NewFile(4, "source-transition-original-channel")
	if raw == nil {
		return nil, true, ErrInput
	}
	typeCode, te := syscall.GetsockoptInt(int(raw.Fd()), syscall.SOL_SOCKET, syscall.SO_TYPE)
	credentialCode, ce := syscall.GetsockoptInt(int(raw.Fd()), syscall.SOL_SOCKET, syscall.SO_PASSCRED)
	if te != nil || ce != nil || typeCode != syscall.SOCK_SEQPACKET || credentialCode != 1 {
		return nil, true, errors.Join(ErrBinding, te, ce, raw.Close())
	}
	connection, e := net.FileConn(raw)
	closeErr := raw.Close()
	if e != nil {
		return nil, true, errors.Join(e, closeErr)
	}
	channel, ok := connection.(*net.UnixConn)
	if !ok {
		return nil, true, errors.Join(ErrBinding, closeErr, connection.Close())
	}
	c.channel = channel
	if closeErr != nil {
		return nil, true, closeErr
	}
	// The original root must deliver hello within the finite startup budget.
	// This timer is removed BEFORE the coordinator context/progress is exposed.
	if err = c.channel.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return nil, true, err
	}
	hello, e := readSourceTransitionMessage(c.channel, c.parent)
	if e != nil {
		return nil, true, e
	}
	wanted := sourceTransitionMessage{Schema: "ardents-source-transition-hello-v1", Nonce: c.nonce, Operation: operation, Deadline: hello.Deadline}
	deadline := time.Unix(0, hello.Deadline)
	if hello != wanted || !deadline.After(time.Now()) || deadline.After(time.Now().Add(2*time.Minute)) {
		return nil, true, ErrBinding
	}
	// The actual parent must execute the same retained artifact, not merely know
	// this process's nonce or supply a same-looking argv.
	parentProgram, e := os.Open("/proc/" + strconv.Itoa(int(c.parent)) + "/exe")
	if e != nil {
		return nil, true, e
	}
	parentInfo, e := parentProgram.Stat()
	parentClosed := parentProgram.Close()
	if e != nil || parentClosed != nil || !os.SameFile(selfInfo, parentInfo) || int32(os.Getppid()) != c.parent {
		return nil, true, errors.Join(ErrBinding, e, parentClosed)
	}
	if err = writeSourceTransitionMessage(c.channel, sourceTransitionMessage{Schema: "ardents-source-transition-ready-v1", Nonce: c.nonce, Operation: operation}); err != nil {
		return nil, true, err
	}
	if err = c.channel.SetDeadline(time.Time{}); err != nil {
		return nil, true, err
	}
	base, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.ctx = context.WithValue(sourceTransitionDeadline{Context: base, deadline: deadline}, sourceTransitionContextKey{}, c)
	return c, true, nil
}

func (c *SourceTransitionCoordinator) Context() context.Context {
	if c == nil {
		return nil
	}
	return c.ctx
}

func (c *SourceTransitionCoordinator) report(b *sourceBirths) (err error) {
	if c == nil || c.closed || c.terminal != nil || b == nil || b.journal == nil || c.sequence == ^uint32(0) || int32(os.Getppid()) != c.parent {
		return ErrInput
	}
	defer func() {
		if err != nil {
			c.terminal = errors.Join(c.terminal, err)
		}
	}()
	if err = c.ctx.Err(); err != nil {
		return err
	}
	if err = b.journal.check(); err != nil {
		return err
	}
	native := b.journal.info.Sys().(*syscall.Stat_t)
	c.sequence++
	message := sourceTransitionMessage{Schema: "ardents-source-transition-boundary-v1", Nonce: c.nonce, Sequence: c.sequence, Operation: c.operation,
		Journal: b.journal.path, Device: uint64(native.Dev), Inode: native.Ino, PrefixSHA256: digest(b.journal.body), Records: b.sequence, Generation: b.transitionGeneration, BindingSHA256: b.transitionBinding, ManagerIdentity: b.transitionManager}
	if err = writeSourceTransitionMessage(c.channel, message); err != nil {
		return err
	}
	// Only this successful-send path can stop. The original parent must verify
	// the actual group stop before admitting quiescence. No cleanup is attempted
	// between the report and this uncatchable self-stop.
	if err = syscall.Kill(os.Getpid(), syscall.SIGSTOP); err != nil {
		return err
	}
	ack, err := readSourceTransitionMessage(c.channel, c.parent)
	if err != nil {
		return err
	}
	if ack != (sourceTransitionMessage{Schema: "ardents-source-transition-ack-v1", Nonce: c.nonce, Sequence: c.sequence, Operation: c.operation}) {
		return ErrBinding
	}
	return c.ctx.Err()
}

func (c *SourceTransitionCoordinator) Close() error {
	if c == nil {
		return nil
	}
	if c.closed {
		return c.terminal
	}
	c.closed = true
	if c.cancel != nil {
		c.cancel()
	}
	if c.channel != nil {
		c.terminal = errors.Join(c.terminal, c.channel.Close())
	}
	return c.terminal
}

func writeSourceTransitionMessage(channel *net.UnixConn, message sourceTransitionMessage) error {
	raw, err := json.Marshal(message)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if channel == nil || len(raw) > sourceTransitionPacketLimit {
		return ErrInput
	}
	n, oob, err := channel.WriteMsgUnix(raw, nil, nil)
	if err != nil || n != len(raw) || oob != 0 {
		return errors.Join(io.ErrShortWrite, err)
	}
	return nil
}

func readSourceTransitionMessage(channel *net.UnixConn, peer int32) (sourceTransitionMessage, error) {
	if channel == nil || peer <= 0 {
		return sourceTransitionMessage{}, ErrInput
	}
	raw := make([]byte, sourceTransitionPacketLimit+1)
	control := make([]byte, syscall.CmsgSpace(syscall.SizeofUcred))
	n, cn, flags, _, err := channel.ReadMsgUnix(raw, control)
	messages, controlErr := syscall.ParseSocketControlMessage(control[:cn])
	// recvmsg may install real descriptors even in a refused/truncated packet.
	// Release every delivered right before checking framing or credentials.
	// The kernel closes rights that did not fit the bounded ancillary buffer.
	rights := false
	var releaseErr error
	for _, message := range messages {
		if message.Header.Level != syscall.SOL_SOCKET || message.Header.Type != syscall.SCM_RIGHTS {
			continue
		}
		rights = true
		fds, e := syscall.ParseUnixRights(&message)
		releaseErr = errors.Join(releaseErr, e)
		for _, fd := range fds {
			releaseErr = errors.Join(releaseErr, syscall.Close(fd))
		}
	}
	if err != nil || controlErr != nil || rights || n <= 0 || n > sourceTransitionPacketLimit || flags&(syscall.MSG_TRUNC|syscall.MSG_CTRUNC) != 0 || len(messages) != 1 {
		return sourceTransitionMessage{}, errors.Join(ErrBinding, err, controlErr, releaseErr)
	}
	credentials, err := syscall.ParseUnixCredentials(&messages[0])
	if err != nil || credentials.Pid != peer || credentials.Uid != 0 || credentials.Gid != 0 {
		return sourceTransitionMessage{}, errors.Join(ErrBinding, err)
	}
	var message sourceTransitionMessage
	if json.Unmarshal(raw[:n], &message) != nil {
		return sourceTransitionMessage{}, ErrBinding
	}
	canonical, err := json.Marshal(message)
	if err != nil || !bytes.Equal(raw[:n], append(canonical, '\n')) {
		return sourceTransitionMessage{}, errors.Join(ErrBinding, err)
	}
	return message, nil
}
