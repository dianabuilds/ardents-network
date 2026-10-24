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

// Go may resume a SIGURG-interrupted preemption prologue at function entry.
// A hardware entry trap is therefore not necessarily a new ACK invocation.
// Retain the actual signal-at-entry cause, original Go caller and exact native
// journal image. Journal record counts alone never deduplicate an entry.
type sourceCutACKEntry struct {
	registers syscall.PtraceRegs
	caller    [8]byte
	raw       []byte
	info      os.FileInfo
}

type sourceCutACKObserver struct {
	previous    map[uint64]sourceCutACKEntry
	interrupted map[uint64]sourceCutACKEntry
}

// A Go caller held at an ACK entry still owns its P. A genuine pending
// preemption must run before another goroutine can finish a GC world stop and
// acknowledge cancellation. The caller is resumed only for that real signal;
// its subsequent entry still requires the original signal/caller/image replay
// proof and is held again before any ACK effect.
func sourceCutACKPendingPreemption(tid int, entry uint64) (syscall.PtraceRegs, bool, error) {
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
			return registers, false, errors.New("held original ACK moved before pending preemption")
		}
		return registers, true, nil
	}
	return registers, false, errors.New("original ACK thread pending signals absent")
}

func readSourceCutACKEntry(tid int, registers syscall.PtraceRegs, journal string) (sourceCutACKEntry, error) {
	entry := sourceCutACKEntry{registers: registers}
	n, err := syscall.PtracePeekData(tid, uintptr(registers.Rsp), entry.caller[:])
	if err != nil || n != len(entry.caller) {
		return entry, errors.Join(errors.New("original ACK caller return address unavailable"), err)
	}
	entry.raw, entry.info, err = readSourceCutObserverFile(journal, 4<<20)
	return entry, err
}

func (e sourceCutACKEntry) same(other sourceCutACKEntry) bool {
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

func (o *sourceCutACKObserver) interrupt(tid int, registers syscall.PtraceRegs, journal string) error {
	prior, ok := o.previous[registers.R14]
	delete(o.interrupted, registers.R14)
	if !ok {
		return nil
	}
	current, err := readSourceCutACKEntry(tid, registers, journal)
	if err != nil {
		return err
	}
	if prior.same(current) {
		if o.interrupted == nil {
			o.interrupted = make(map[uint64]sourceCutACKEntry)
		}
		o.interrupted[registers.R14] = current
	}
	return nil
}

func (o *sourceCutACKObserver) enter(tid int, registers syscall.PtraceRegs, journal string) (bool, error) {
	current, err := readSourceCutACKEntry(tid, registers, journal)
	if err != nil {
		return false, err
	}
	prior, interrupted := o.interrupted[registers.R14]
	delete(o.interrupted, registers.R14)
	if interrupted && prior.same(current) {
		return true, nil
	}
	if o.previous == nil {
		o.previous = make(map[uint64]sourceCutACKEntry)
	}
	if _, exists := o.previous[registers.R14]; !exists && len(o.previous) >= 256 {
		return false, errors.New("original ACK caller bound exceeded")
	}
	o.previous[registers.R14] = current
	return false, nil
}
