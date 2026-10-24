//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func (p *sourceRefusalObserverProfile) validEarlyPrefixObservation(root string) bool {
	return p != nil && (p.EarlyInitialPrefix == 24 || p.EarlyInitialPrefix == 42) &&
		p.Schema == "source-early-prefix-recovery-refusal-observer-v1" && len(p.Arguments) == 8 &&
		p.Arguments[7] == root+".source-initial-journal" && p.JournalMode == "0600" &&
		p.StartupPending == nil && p.RetirementPending == nil && p.RecoveryHold == nil && !p.MissingOriginalCut &&
		!p.ExternalJournalLease && !p.ExternalReleaseLease
}

func sourceEarlyPrefixNativeCase(scenario string) (string, int) {
	for _, count := range []int{24, 42} {
		for _, kind := range []string{"read", "unattempted", "unlocked", "partial", "foreign", "released", "shared"} {
			if scenario == "refusal-early-"+kind+"-"+strconv.Itoa(count) {
				return kind, count
			}
		}
	}
	return "", 0
}

type sourceEarlyPrefixCall struct {
	number, address, count uint64
	fd                     int
	position               int64
	exclusive              bool
}

// This observes a successful fresh journal lease and actual complete read,
// not an externally held lease or missing-cut lookup. It grants no continuation.
type sourceEarlyPrefix struct {
	path, pin string
	count     int
	raw       []byte
	info      os.FileInfo
	calls     map[int]sourceEarlyPrefixCall
	locked    map[int]int64
	read      bool
	invalid   bool
}

func newSourceEarlyPrefix(root, scenario string, profile *sourceRefusalObserverProfile) *sourceEarlyPrefix {
	_, count := sourceEarlyPrefixNativeCase(scenario)
	path, pin := filepath.Join(root, "original-journal"), ""
	if profile != nil {
		if profile.EarlyInitialPrefix == 0 {
			return nil
		}
		path, pin, count = profile.Arguments[7], profile.JournalSHA256, profile.EarlyInitialPrefix
	}
	if count == 0 {
		return nil
	}
	return &sourceEarlyPrefix{path: path, pin: pin, count: count, calls: map[int]sourceEarlyPrefixCall{}, locked: map[int]int64{}}
}

func (p *sourceEarlyPrefix) capture() error {
	raw, info, err := readSourceCutObserverFile(p.path, 4<<20)
	if err != nil || !sourceAccess(info, 0600, 0, 0, false) || p.pin != "" && sourcePhysicalSHA(raw) != p.pin {
		return errors.Join(errors.New("original early prefix pin/access required"), err)
	}
	records, err := decodeSourceRecoveryPrefix(raw, sourceRecoverableInitialRecords)
	if err != nil || len(records) != p.count || sourceInitialRecoveryPrefixSupported(records) {
		return errors.Join(errors.New("exact unsupported early initial prefix required"), err)
	}
	native := info.Sys().(*syscall.Stat_t)
	if records[0] != (sourceRecord{Phase: sourceRecoverableInitialPhase, Path: p.path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}) {
		return errors.New("original early prefix header/native lineage required")
	}
	p.raw, p.info, p.pin = raw, info, sourcePhysicalSHA(raw)
	return probeSourceRefusalLease(p.path, p.pin, 0600, false)
}

func sourceEarlyPrefixPosition(pid, fd int) (int64, error) {
	file, err := os.OpenFile(filepath.Join("/proc", strconv.Itoa(pid), "fdinfo", strconv.Itoa(fd)), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return 0, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, (16<<10)+1))
	if err := errors.Join(readErr, file.Close()); err != nil || len(raw) > 16<<10 {
		return 0, errors.Join(errors.New("bounded original fdinfo required"), err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "pos:") {
			position, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "pos:")), 10, 64)
			if err != nil || position < 0 {
				return 0, errors.Join(errors.New("actual native journal position required"), err)
			}
			return position, nil
		}
	}
	return 0, errors.New("actual native journal position missing")
}

func (p *sourceEarlyPrefix) fd(pid, fd int) bool {
	info, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid), "fd", strconv.Itoa(fd)))
	return err == nil && p.info != nil && sameSourceFile(p.info, info)
}

func (p *sourceEarlyPrefix) syscall(pid, tid int, result *sourceCutObservation) error {
	var registers syscall.PtraceRegs
	if err := syscall.PtraceGetRegs(tid, &registers); err != nil {
		return err
	}
	number := registers.Orig_rax
	if number != syscall.SYS_FLOCK && number != syscall.SYS_READ && number != syscall.SYS_CLOSE {
		return nil
	}
	if int64(registers.Rax) == -int64(syscall.ENOSYS) {
		fd := int(registers.Rdi)
		if !p.fd(pid, fd) {
			return nil
		}
		call := sourceEarlyPrefixCall{number: number, fd: fd}
		switch number {
		case syscall.SYS_FLOCK:
			call.exclusive = registers.Rsi == syscall.LOCK_EX|syscall.LOCK_NB
			if !call.exclusive {
				if _, held := p.locked[fd]; !held {
					return nil
				}
			}
		case syscall.SYS_READ:
			if _, ok := p.locked[fd]; !ok || p.read {
				return nil
			}
			if registers.Rdx == 0 || registers.Rdx > (4<<20)+1 {
				return errors.New("bounded actual original journal read required")
			}
			var err error
			call.position, err = sourceEarlyPrefixPosition(pid, fd)
			if err != nil {
				return err
			}
			call.address, call.count = registers.Rsi, registers.Rdx
		case syscall.SYS_CLOSE:
			if _, ok := p.locked[fd]; !ok {
				return nil
			}
		}
		if _, exists := p.calls[tid]; exists {
			return errors.New("early prefix syscall entry repeated without paired exit")
		}
		p.calls[tid] = call
		return nil
	}
	call, exists := p.calls[tid]
	if !exists {
		return nil
	}
	delete(p.calls, tid)
	if call.number != number || int64(registers.Rax) < 0 {
		return fmt.Errorf("original early prefix syscall did not complete: %d", int64(registers.Rax))
	}
	switch number {
	case syscall.SYS_FLOCK:
		if registers.Rax != 0 || !p.fd(pid, call.fd) {
			return errors.New("original successful fresh journal flock missing")
		}
		if !call.exclusive {
			// A successful unlock or shared downgrade cannot retain the earlier
			// exclusive custody. Even later reacquisition cannot erase it.
			delete(p.locked, call.fd)
			p.invalid = true
			return nil
		}
		p.locked[call.fd] = 0
	case syscall.SYS_CLOSE:
		if registers.Rax != 0 {
			return errors.New("actual original journal close did not complete")
		}
		delete(p.locked, call.fd)
	case syscall.SYS_READ:
		length := int64(registers.Rax)
		if !p.fd(pid, call.fd) || length > int64(call.count) || call.position != p.locked[call.fd] || call.position+length > int64(len(p.raw)) {
			return errors.New("original journal read/native position differs")
		}
		if length > 0 {
			raw := make([]byte, int(length))
			n, err := syscall.PtracePeekData(tid, uintptr(call.address), raw)
			if err != nil || n != len(raw) || !bytes.Equal(raw, p.raw[call.position:call.position+length]) {
				return errors.Join(errors.New("actual original journal read bytes differ"), err)
			}
		}
		p.locked[call.fd] = call.position + length
		if p.locked[call.fd] == int64(len(p.raw)) {
			p.read = true
			native := p.info.Sys().(*syscall.Stat_t)
			result.RefusalEarlyPrefixObserved, result.RefusalEarlyPrefixRecords = true, p.count
			result.RefusalEarlyPrefixDevice, result.RefusalEarlyPrefixInode = uint64(native.Dev), native.Ino
			result.RefusalEarlyPrefixSHA256 = p.pin
		}
	}
	return nil
}

func (p *sourceEarlyPrefix) verify(result *sourceCutObservation) error {
	if p.invalid || !p.read || len(p.calls) != 0 || len(p.locked) != 0 {
		return errors.New("original fresh-prefix lease/full read/close not observed before return")
	}
	raw, info, err := readSourceCutObserverFile(p.path, 4<<20)
	if err != nil || !sameSourceFile(p.info, info) || !bytes.Equal(raw, p.raw) {
		return errors.Join(errors.New("original early prefix changed during refusal"), err)
	}
	result.RefusalEarlyPrefixClosed = true
	return nil
}
