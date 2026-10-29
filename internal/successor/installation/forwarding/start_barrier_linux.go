package forwarding

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
)

type startBarrier struct {
	owner                                 *startAttempt
	guard, record, archive                *initialFile
	socket                                os.FileInfo
	listener                              *net.UnixListener
	connection                            *net.UnixConn
	peerAttempted, accepted, ackAttempted bool
	terminal                              error
}

type startIntent struct {
	Schema               string `json:"schema"`
	BindingSHA256        string `json:"binding_sha256"`
	InitialHistorySHA256 string `json:"initial_history_sha256"`
	TargetsSHA256        string `json:"targets_sha256"`
	UID                  uint32 `json:"uid"`
	GID                  uint32 `json:"gid"`
	Attempt              uint32 `json:"attempt"`
}
type startSocketRecord struct {
	Schema      string `json:"schema"`
	GuardSHA256 string `json:"guard_sha256"`
	Device      uint64 `json:"device"`
	Inode       uint64 `json:"inode"`
	GID         uint32 `json:"gid"`
}

func prepareStartBarrier(owner *startAttempt) (barrier *startBarrier, err error) {
	if owner == nil || owner.storage == nil {
		return nil, ErrInput
	}
	barrier = &startBarrier{owner: owner}
	s := owner.storage
	b := owner.completed.original.binding
	admit := owner.observeStopped
	if err = s.birthDirectory(b.GID, admit); err != nil {
		return barrier, err
	}
	binding := owner.bindingBytes()
	if _, err = s.write("forwarding-start-pending", []byte("ardents-forwarding-start-pending-v1\n"+digest(binding)+"\n"), admit); err != nil {
		return barrier, err
	}
	guard, err := owner.startIntentBytes()
	if err != nil || len(owner.intent) == 0 || !bytes.Equal(guard, owner.intent) {
		return barrier, errors.Join(ErrBinding, err)
	}
	barrier.guard, err = s.write("start-guard.json", owner.intent, admit)
	if err != nil {
		return barrier, err
	}
	path := filepath.Join(s.path, "start-completion.socket")
	if err = s.record("socket-intent", path, nil, ""); err != nil {
		return barrier, err
	}
	if err = admit(); err != nil {
		return barrier, err
	}
	if _, e := os.Lstat(path); !errors.Is(e, os.ErrNotExist) {
		return barrier, errors.Join(ErrBinding, e)
	}
	barrier.listener, err = net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return barrier, err
	}
	barrier.listener.SetUnlinkOnClose(false)
	born, err := os.Lstat(path)
	if err != nil || born.Mode()&os.ModeSocket == 0 {
		return barrier, errors.Join(ErrBinding, err)
	}
	native, ok := born.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid != 0 || native.Nlink != 1 {
		return barrier, ErrBinding
	}
	if err = s.record("socket-born", path, born, ""); err != nil {
		return barrier, err
	}
	if err = admit(); err != nil {
		return barrier, err
	}
	current, e := os.Lstat(path)
	if e != nil || !sameStartSocket(born, current, born.Mode().Perm(), 0) {
		return barrier, errors.Join(ErrBinding, e)
	}
	if err = os.Lchown(path, 0, int(b.GID)); err != nil {
		return barrier, err
	}
	current, e = os.Lstat(path)
	if e != nil || !os.SameFile(born, current) || current.Mode()&os.ModeSocket == 0 {
		return barrier, errors.Join(ErrBinding, e)
	}
	if err = os.Chmod(path, 0660); err != nil {
		return barrier, err
	}
	barrier.socket, err = os.Lstat(path)
	if err != nil || !os.SameFile(born, barrier.socket) || !sameStartSocket(barrier.socket, barrier.socket, 0660, b.GID) {
		return barrier, errors.Join(ErrBinding, err)
	}
	if err = s.dir.Sync(); err != nil {
		return barrier, err
	}
	if err = s.record("socket-ready", path, barrier.socket, ""); err != nil {
		return barrier, err
	}
	native = barrier.socket.Sys().(*syscall.Stat_t)
	socketSchema := "ardents-forwarding-start-socket-v1"
	raw, err := json.Marshal(startSocketRecord{socketSchema, digest(barrier.guard.body), uint64(native.Dev), native.Ino, b.GID})
	if err != nil {
		return barrier, err
	}
	barrier.record, err = s.write("start-socket.json", append(raw, '\n'), admit)
	if err != nil {
		return barrier, err
	}
	return barrier, barrier.observe()
}

// The actual root start owns the fresh decision and complete initial/prior
// history. These bytes are fixed before any start journal effect and reused by
// the guard writer and original cut; recomputation may only detect refusal.
func (s *startAttempt) startIntentBytes() ([]byte, error) {
	c := s.completed.original
	program, _ := s.authorization.Targets()
	decision, ok := program.AcceptedDecision()
	if c == nil || !ok || len(decision.Floors.TargetsDigest) != 32 {
		return nil, ErrBinding
	}
	return encodeHistoryStartIntent(c.binding, digest(s.bindingBytes()), c.initialHistorySHA(), hex.EncodeToString(decision.Floors.TargetsDigest), s.nextNumber, s.predecessor, s.historyOpening)
}

func sameStartSocket(original, current os.FileInfo, mode os.FileMode, gid uint32) bool {
	if original == nil || current == nil || !os.SameFile(original, current) || current.Mode() != os.ModeSocket|mode {
		return false
	}
	a, ok := original.Sys().(*syscall.Stat_t)
	b, yes := current.Sys().(*syscall.Stat_t)
	return ok && yes && b.Uid == 0 && b.Gid == gid && b.Nlink == 1 && a.Ctim == b.Ctim && a.Mtim == b.Mtim
}

func (b *startBarrier) observe() error {
	if b == nil || b.owner == nil || b.listener == nil || b.guard == nil || b.record == nil {
		return ErrBinding
	}
	if err := b.owner.ctx.Err(); err != nil {
		return err
	}
	if err := observeInitialFile(b.guard); err != nil {
		return err
	}
	if err := observeInitialFile(b.record); err != nil {
		return err
	}
	current, err := os.Lstat(filepath.Join(b.owner.storage.path, "start-completion.socket"))
	if err != nil || !sameStartSocket(b.socket, current, 0660, b.owner.completed.original.binding.GID) {
		return errors.Join(ErrBinding, err)
	}
	raw, err := b.listener.SyscallConn()
	if err != nil {
		return err
	}
	var nativeErr error
	err = raw.Control(func(fd uintptr) {
		kind, e := syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_TYPE)
		accepting, ae := syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_ACCEPTCONN)
		if e != nil || ae != nil || kind != syscall.SOCK_STREAM || accepting != 1 {
			nativeErr = errors.Join(ErrBinding, e, ae)
		}
	})
	return errors.Join(err, nativeErr, b.owner.ctx.Err())
}

func (b *startBarrier) frame() ([160]byte, error) {
	s := b.owner
	binding := s.bindingBytes()
	return completion.Encode(s.id, s.completed.original.binding.Generation, digest(binding))
}

func (b *startBarrier) peer() error {
	if b.connection == nil {
		return ErrInput
	}
	raw, err := b.connection.SyscallConn()
	if err != nil {
		return err
	}
	var peer *syscall.Ucred
	var nativeErr error
	err = raw.Control(func(fd uintptr) {
		peer, nativeErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	bound := b.owner.completed.original.binding
	if err != nil || nativeErr != nil || peer == nil || peer.Pid <= 0 || uint32(peer.Pid) != b.owner.pid || peer.Uid != bound.UID || peer.Gid != bound.GID {
		return errors.Join(ErrBinding, err, nativeErr)
	}
	return b.owner.ctx.Err()
}

func (b *startBarrier) accept() (err error) {
	if b == nil || b.owner == nil || b.peerAttempted {
		return ErrInput
	}
	s := b.owner
	if err = s.observeRunning(); err != nil {
		return err
	}
	b.peerAttempted = true
	deadline, ok := s.ctx.Deadline()
	if !ok {
		return ErrInput
	}
	if err = b.listener.SetDeadline(deadline); err != nil {
		return err
	}
	listener := b.listener
	closed := make(chan error, 1)
	stop := context.AfterFunc(s.ctx, func() { closed <- listener.Close() })
	defer func() {
		if !stop() {
			err = errors.Join(err, <-closed)
		}
	}()
	b.connection, err = listener.AcceptUnix()
	if err != nil {
		return err
	}
	connection := b.connection
	if err = connection.SetDeadline(deadline); err != nil {
		return err
	}
	connectionClosed := make(chan error, 1)
	stopConnection := context.AfterFunc(s.ctx, func() { connectionClosed <- connection.Close() })
	defer func() {
		if !stopConnection() {
			err = errors.Join(err, <-connectionClosed)
		}
	}()
	if err = s.observeRunning(); err != nil {
		return err
	}
	if err = b.peer(); err != nil {
		return err
	}
	frame, err := b.frame()
	if err != nil {
		return err
	}
	var actual [160]byte
	if _, err = io.ReadFull(connection, actual[:]); err != nil {
		return err
	}
	if actual != frame {
		return ErrBinding
	}
	if err = b.peer(); err != nil {
		return err
	}
	if err = s.observeRunning(); err != nil {
		return err
	}
	if err = s.storage.record("completion-received", s.storage.path, nil, digest(frame[:])); err != nil {
		return err
	}
	if err = s.observeRunning(); err != nil {
		return err
	}
	b.accepted = true
	return s.ctx.Err()
}

type startArchive struct {
	Schema           string `json:"schema"`
	GuardSHA256      string `json:"guard_sha256"`
	BindingSHA256    string `json:"binding_sha256"`
	Generation       string `json:"generation"`
	PID              uint32 `json:"pid"`
	InvocationID     string `json:"invocation_id"`
	ProcessDevice    uint64 `json:"process_device"`
	ProcessInode     uint64 `json:"process_inode"`
	CgroupDevice     uint64 `json:"cgroup_device"`
	CgroupInode      uint64 `json:"cgroup_inode"`
	Manager          string `json:"manager"`
	StartedRealtime  uint64 `json:"started_realtime"`
	StartedMonotonic uint64 `json:"started_monotonic"`
}

func (s *startAttempt) archive() error {
	if s.barrier == nil || !s.barrier.accepted || s.barrier.archive != nil {
		return ErrBinding
	}
	if err := s.observeRunning(); err != nil {
		return err
	}
	process, err := os.Lstat(filepath.Join("/proc", strconv.FormatUint(uint64(s.pid), 10)))
	if err != nil || !process.IsDir() {
		return errors.Join(ErrBinding, err)
	}
	scope, err := os.Lstat("/sys/fs/cgroup/system.slice/ardents-forwarding.service")
	if err != nil || !scope.IsDir() {
		return errors.Join(ErrBinding, err)
	}
	p, ok := process.Sys().(*syscall.Stat_t)
	c, yes := scope.Sys().(*syscall.Stat_t)
	if !ok || !yes {
		return ErrBinding
	}
	facts, err := s.manager.Observe()
	if err != nil {
		return err
	}
	execution, err := forwardingExecutionStatus(facts.Service)
	if err != nil {
		return err
	}
	if err = s.observeRunning(); err != nil {
		return err
	}
	b := s.completed.original.binding
	binding := s.bindingBytes()
	archiveSchema := "ardents-forwarding-start-archive-v1"
	// Retain the full broker-instance/unique-manager identity already freshly
	// rechecked by observeRunning. Observe's Manager is only the unique owner;
	// it cannot match the independent full identity required by Stop/restart.
	archive := startArchive{archiveSchema, digest(s.barrier.guard.body), digest(binding), b.Generation, s.pid, hex.EncodeToString(s.id[:]), uint64(p.Dev), p.Ino, uint64(c.Dev), c.Ino, s.managerIdentity, execution.startedRealtime, execution.startedMonotonic}
	raw, err := json.Marshal(archive)
	if err != nil {
		return err
	}
	original, err := s.storage.write("start-archived.json", append(raw, '\n'), s.observeRunning)
	if err != nil {
		return err
	}
	if err = s.storage.record("archived", original.path, original.info, digest(original.body)); err != nil {
		return err
	}
	if err = s.observeRunning(); err != nil {
		return err
	}
	s.barrier.archive = original
	return s.ctx.Err()
}

func (b *startBarrier) acknowledge() (err error) {
	if b == nil || !b.accepted || b.archive == nil || b.ackAttempted || b.connection == nil {
		return ErrBinding
	}
	s := b.owner
	if err = s.observeRunning(); err != nil {
		return err
	}
	if err = observeInitialFile(b.archive); err != nil {
		return err
	}
	if err = b.peer(); err != nil {
		return err
	}
	frame, err := b.ackFrame()
	if err != nil {
		return err
	}
	if err = s.storage.record("ack-attempt", s.storage.path, nil, digest(frame[:])); err != nil {
		return err
	}
	if err = s.observeRunning(); err != nil {
		return err
	}
	b.ackAttempted = true
	connection := b.connection
	closed := make(chan error, 1)
	stop := context.AfterFunc(s.ctx, func() { closed <- connection.Close() })
	defer func() {
		if !stop() {
			err = errors.Join(err, <-closed)
		}
	}()
	count, err := io.Copy(connection, bytes.NewReader(frame[:]))
	if count == int64(len(frame)) {
		s.handed = true
	}
	if err != nil || count != int64(len(frame)) {
		return errors.Join(io.ErrShortWrite, err)
	}
	if err = s.storage.record("ack-written", s.storage.path, nil, digest(frame[:])); err != nil {
		return err
	}
	return s.ctx.Err()
}

func (b *startBarrier) ackFrame() ([161]byte, error) {
	if b == nil || b.owner == nil {
		return [161]byte{}, ErrInput
	}
	request, err := b.frame()
	if err != nil {
		return [161]byte{}, err
	}
	return completion.EncodeForwardingACK(request, b.owner.historyOpening)
}

func (b *startBarrier) close() error {
	if b == nil {
		return nil
	}
	if b.connection != nil {
		err := b.connection.Close()
		if !errors.Is(err, net.ErrClosed) {
			b.terminal = errors.Join(b.terminal, err)
		}
		b.connection = nil
	}
	if b.listener != nil {
		err := b.listener.Close()
		if !errors.Is(err, net.ErrClosed) {
			b.terminal = errors.Join(b.terminal, err)
		}
		b.listener = nil
	}
	// Retain the directory, records and stale socket. Independent Startup still
	// observes the original directory after full ACK; no restart is authorized.
	return b.terminal
}
