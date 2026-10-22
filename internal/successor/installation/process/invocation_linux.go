package process

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var errInput = errors.New("installation process: invalid input")

// Expected describes already checked fixed Installation facts. It conveys no
// authority. Retain snapshots these values and never accepts a replacement PID,
// executable inode, generation or InvocationID through Observe.
type Expected struct {
	PID, UID, GID                uint32
	Invocation                   [16]byte
	InstallationRoot, Generation string
	ProgramIdentity              os.FileInfo
}

// Invocation owns one original proc directory and its start clock. Installation
// must retain it until the separately owned original cgroups have joined.
type Invocation struct {
	root                        *os.Root
	identity                    os.FileInfo
	pid, uid, gid               uint32
	invocation                  [16]byte
	started                     uint64
	program                     string
	arguments, placement        string
	programDevice, programInode uint64
	namespaces                  map[string]*os.File // Source only; original kernel objects
	sourceSignal                *sourceSignal       // shared original root Source signal lifetime
}

func Retain(ctx context.Context, expected Expected) (result *Invocation, returnedErr error) {
	if os.Geteuid() != 0 {
		return nil, errInput
	}
	return retain(ctx, expected)
}

// RetainSelf observes only the calling non-root service process. It cannot
// inspect another PID or replace the privileged predecessor opening. Exact
// executable, arguments, kernel protection and manager cgroup checks still run.
func RetainSelf(ctx context.Context, expected Expected) (*Invocation, error) {
	if os.Geteuid() == 0 || expected.PID != uint32(os.Getpid()) ||
		expected.UID != uint32(os.Geteuid()) || expected.GID != uint32(os.Getegid()) {
		return nil, errInput
	}
	return retain(ctx, expected)
}

func retain(ctx context.Context, expected Expected) (result *Invocation, returnedErr error) {
	return retainFixed(ctx, expected, endpointInvocationPolicy(expected))
}

// A closed policy is chosen only by the separate fixed-purpose openers. No
// public input can select a program name, argument vector, unit or cgroup.
type invocationPolicy struct {
	program, arguments, placement string
}

func endpointInvocationPolicy(expected Expected) invocationPolicy {
	program := filepath.Join(expected.InstallationRoot, "generations", expected.Generation, "ardents-linux-amd64")
	return invocationPolicy{program: program,
		arguments: strings.Join([]string{program, "endpoint", "start-installed", expected.InstallationRoot, ""}, "\x00"),
		placement: "0::/system.slice/ardents-endpoint.service\n"}
}

func retainFixed(ctx context.Context, expected Expected, policy invocationPolicy) (result *Invocation, returnedErr error) {
	if ctx == nil || expected.PID == 0 || expected.UID == 0 || expected.GID == 0 ||
		expected.UID == ^uint32(0) || expected.GID == ^uint32(0) ||
		expected.Invocation == [16]byte{} ||
		!filepath.IsAbs(expected.InstallationRoot) || filepath.Clean(expected.InstallationRoot) != expected.InstallationRoot ||
		expected.InstallationRoot == "/" || strings.ContainsRune(expected.InstallationRoot, 0) ||
		expected.ProgramIdentity == nil {
		return nil, errInput
	}
	digest, err := hex.DecodeString(expected.Generation)
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != expected.Generation {
		return nil, errInput
	}
	native, ok := expected.ProgramIdentity.Sys().(*syscall.Stat_t)
	if !ok || native.Ino == 0 || native.Dev == 0 || !expected.ProgramIdentity.Mode().IsRegular() {
		return nil, errInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(filepath.Join("/proc", strconv.FormatUint(uint64(expected.PID), 10)))
	if err != nil {
		return nil, err
	}
	observed := &Invocation{root: root, pid: expected.PID, uid: expected.UID, gid: expected.GID,
		invocation: expected.Invocation,
		program:    policy.program, arguments: policy.arguments, placement: policy.placement,
		programDevice: uint64(native.Dev), programInode: native.Ino}
	defer func() {
		if returnedErr != nil {
			returnedErr = errors.Join(returnedErr, observed.Close())
			result = nil
		}
	}()
	observed.identity, err = root.Stat(".")
	if err != nil || !observed.identity.IsDir() {
		return nil, errors.Join(errBinding, err)
	}
	if err := observed.Observe(ctx); err != nil {
		return nil, err
	}
	return observed, nil
}

func (pin *Invocation) read(ctx context.Context, name string, maximum int64) (body []byte, returnedErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := pin.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, file.Close(), ctx.Err())
		if returnedErr != nil {
			body = nil
		}
	}()
	body, err = io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || len(body) == 0 || int64(len(body)) > maximum {
		return nil, errors.Join(errBinding, err)
	}
	return body, nil
}

// Observe checks only the same retained kernel process. An uncancelled cleanup
// context may inspect it, but grants no new pin, caller or startup authority.
func (pin *Invocation) Observe(ctx context.Context) error {
	if pin == nil || ctx == nil || pin.root == nil || pin.identity == nil {
		return errInput
	}
	if err := pin.observeSourceNamespaces(ctx); err != nil {
		return err
	}
	before, err := pin.root.Stat(".")
	if err != nil || !os.SameFile(pin.identity, before) {
		return errors.Join(errBinding, err)
	}
	stat, err := pin.read(ctx, "stat", 4<<10)
	if err != nil {
		return err
	}
	started, err := startClock(stat, pin.pid)
	if err != nil || (pin.started != 0 && pin.started != started) {
		return errors.Join(errBinding, err)
	}
	program := pin.program
	procExe := filepath.Join("/proc", strconv.FormatUint(uint64(pin.pid), 10), "exe")
	// This one kernel symlink is intentionally followed, then matched to the
	// already inspected original program inode and its exact canonical path.
	link, linkErr := os.Readlink(procExe)
	executable, exeErr := os.Stat(procExe)
	observedExe, exeOK := executableNative(executable)
	if linkErr != nil || exeErr != nil || link != program || !exeOK || uint64(observedExe.Dev) != pin.programDevice || observedExe.Ino != pin.programInode {
		return errors.Join(errBinding, linkErr, exeErr)
	}
	arguments, err := pin.read(ctx, "cmdline", 64<<10)
	if err != nil {
		return err
	}
	if !bytes.Equal(arguments, []byte(pin.arguments)) {
		return errBinding
	}
	status, err := pin.read(ctx, "status", 64<<10)
	if err != nil {
		return err
	}
	if credentials(status, pin.uid, pin.gid) != nil {
		return errBinding
	}
	cgroup, err := pin.read(ctx, "cgroup", 4<<10)
	if err != nil {
		return err
	}
	if !bytes.Equal(cgroup, []byte(pin.placement)) {
		return errBinding
	}
	environment, err := pin.read(ctx, "environ", 64<<10)
	if err != nil {
		return err
	}
	if !invocationMatches(environment, pin.invocation) {
		return errBinding
	}
	stat, err = pin.read(ctx, "stat", 4<<10)
	if err != nil {
		return err
	}
	afterStarted, err := startClock(stat, pin.pid)
	after, identityErr := pin.root.Stat(".")
	if err != nil || identityErr != nil || started != afterStarted || !os.SameFile(pin.identity, after) {
		return errors.Join(errBinding, err, identityErr)
	}
	pin.started = started
	return pin.observeSourceNamespaces(ctx)
}

func (pin *Invocation) Close() error {
	if pin == nil || pin.root == nil {
		return nil
	}
	var err error
	if pin.sourceSignal != nil && pin.sourceSignal.file != nil {
		err = pin.sourceSignal.file.Close()
		pin.sourceSignal.file = nil
	}
	for _, file := range pin.namespaces {
		err = errors.Join(err, file.Close())
	}
	err = errors.Join(err, pin.root.Close())
	pin.root = nil
	return err
}

func executableNative(info os.FileInfo) (*syscall.Stat_t, bool) {
	if info == nil {
		return nil, false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return native, ok
}
