//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// A signal at the original Go entry, caller frame and unchanged native image
// jointly distinguish entry preemption from another semantic operation.
type sourceCutGoEntry struct {
	registers syscall.PtraceRegs
	caller    [8]byte
	raw       []byte
	info      os.FileInfo
}

func sourceCutPendingPreemption(tid int, entry uint64) (syscall.PtraceRegs, bool, error) {
	var registers syscall.PtraceRegs
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", tid))
	if err != nil {
		return registers, false, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "SigPnd:") {
			continue
		}
		pending, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "SigPnd:")), 16, 64)
		if err != nil || pending&(1<<(uint(syscall.SIGURG)-1)) == 0 {
			return registers, false, err
		}
		if err := syscall.PtraceGetRegs(tid, &registers); err != nil {
			return registers, false, err
		}
		if registers.Rip != entry {
			return registers, false, errors.New("held original Go entry moved before pending preemption")
		}
		return registers, true, nil
	}
	return registers, false, errors.New("original Go thread pending signals absent")
}

func readSourceCutGoFrame(tid int, registers syscall.PtraceRegs) (sourceCutGoEntry, error) {
	entry := sourceCutGoEntry{registers: registers}
	n, err := syscall.PtracePeekData(tid, uintptr(registers.Rsp), entry.caller[:])
	if err != nil || n != len(entry.caller) {
		return entry, errors.Join(errors.New("original Go caller return address unavailable"), err)
	}
	return entry, nil
}

func readSourceCutGoEntry(tid int, registers syscall.PtraceRegs, journal string) (sourceCutGoEntry, error) {
	entry, err := readSourceCutGoFrame(tid, registers)
	if err != nil {
		return entry, err
	}
	entry.raw, entry.info, err = readSourceCutObserverFile(journal, 4<<20)
	return entry, err
}

// The ACK has three Go ABI arguments. A producer with more arguments also
// compares those arguments before consuming this common caller/image proof.
func (e sourceCutGoEntry) same(other sourceCutGoEntry) bool {
	left, right := e.registers, other.registers
	if left.R14 != right.R14 || left.Rsp != right.Rsp || left.Rbp != right.Rbp ||
		left.Rax != right.Rax || left.Rbx != right.Rbx || left.Rcx != right.Rcx || e.caller != other.caller ||
		e.info == nil || other.info == nil || !os.SameFile(e.info, other.info) || !bytes.Equal(e.raw, other.raw) {
		return false
	}
	a, aOK := e.info.Sys().(*syscall.Stat_t)
	b, bOK := other.info.Sys().(*syscall.Stat_t)
	return aOK && bOK && a.Dev == b.Dev && a.Ino == b.Ino && a.Mode == b.Mode && a.Uid == b.Uid && a.Gid == b.Gid &&
		a.Nlink == b.Nlink && a.Size == b.Size && a.Mtim == b.Mtim && a.Ctim == b.Ctim
}
