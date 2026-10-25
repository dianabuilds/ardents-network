//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"syscall"
)

const sourceRecoveryNativePreemptionFlag = "ARDENTS_SOURCE_RECOVERY_NATIVE_SCHEDULER_HOLD"

// This native-only fixture holds the original M after a real pending SIGURG
// queues its Go caller. Another M must resume that caller before the old M is
// released. It changes no Go memory, journal, cancellation or accepting rule.
type sourceRecoveryNativePreemption struct {
	enabled  bool
	entry    uint64
	original int
	held     int
	released bool
}

func newSourceRecoveryNativePreemption(nativeCase string, installed bool, refusal *sourceRefusalObserverProfile) (sourceRecoveryNativePreemption, error) {
	value := os.Getenv(sourceRecoveryNativePreemptionFlag)
	if value != "" && (value != "1" || installed || refusal != nil || nativeCase != "refusal-recovery-cancel") {
		return sourceRecoveryNativePreemption{}, errors.New("private recovery preemption requires its exact native file/process fixture")
	}
	return sourceRecoveryNativePreemption{enabled: value == "1"}, nil
}

func (p *sourceRecoveryNativePreemption) symbol(symbol elf.Symbol) error {
	if !p.enabled || symbol.Name != "runtime.schedule" {
		return nil
	}
	if p.entry != 0 || elf.ST_TYPE(symbol.Info) != elf.STT_FUNC || symbol.Size == 0 {
		return errors.New("unique original Go scheduler entry required")
	}
	p.entry = symbol.Value
	return nil
}

func (p *sourceRecoveryNativePreemption) validate(address, size uint64) error {
	if p.enabled && (p.entry == 0 || p.entry < address || p.entry >= address+size) {
		return errors.New("original Go scheduler entry missing from executable text")
	}
	return nil
}

func (p *sourceRecoveryNativePreemption) arm(tid int, entry uint64) (bool, error) {
	if !p.enabled {
		return true, sourceCutPtrace(syscall.PTRACE_POKEUSR, tid, sourceCutDebugRegisters+7*8, 1)
	}
	_, pending, err := sourceCutPendingPreemption(tid, entry)
	if err != nil || !pending {
		return false, err
	}
	if p.original != 0 {
		return false, errors.New("original recovery scheduler probe already armed")
	}
	if err := sourceCutPtrace(syscall.PTRACE_POKEUSR, tid, sourceCutDebugRegisters+3*8, uintptr(p.entry)); err != nil {
		return false, err
	}
	// Keep the original return probe in DR0, retire this M's DR2 hold probe,
	// and arm the one-use native scheduler point in DR3.
	if err := sourceCutPtrace(syscall.PTRACE_POKEUSR, tid, sourceCutDebugRegisters+7*8, 65); err != nil {
		return false, err
	}
	p.original = tid
	return true, nil
}

func (p *sourceRecoveryNativePreemption) stop(tid int, registers syscall.PtraceRegs, cancelled bool) (bool, error) {
	if !p.enabled || !cancelled || tid != p.original || registers.Rip != p.entry {
		return false, nil
	}
	if p.held != 0 {
		return false, errors.New("repeated native recovery scheduler stop")
	}
	if _, err := fmt.Printf("SOURCE_RECOVERY_NATIVE_SCHEDULE_ENTRY tid=%d held=%t\n", tid, !p.released); err != nil {
		return false, err
	}
	if p.released {
		// Another M can resume the queued caller before this M reaches its
		// scheduler entry. Do not turn that later stop into an orphaned hold.
		if err := sourceCutPtrace(syscall.PTRACE_POKEUSR, tid, sourceCutDebugRegisters+7*8, 1); err != nil {
			return false, err
		}
		return true, resumeSourceCutBreakpoint(tid, &registers)
	}
	p.held = tid
	return true, nil
}

func (p *sourceRecoveryNativePreemption) release() error {
	p.released = true
	if p.held == 0 {
		return nil
	}
	if err := sourceCutPtrace(syscall.PTRACE_POKEUSR, p.held, sourceCutDebugRegisters+7*8, 1); err != nil {
		return err
	}
	// This fixture selects only the first cancellation caller, which has no
	// exclusion-syscall observer; its ordinary continuation is PTRACE_CONT.
	if err := syscall.PtraceCont(p.held, 0); err != nil {
		return err
	}
	p.held = 0
	return nil
}

// Exercise the replay guard against the actual stopped caller and native
// images, substituting only detached retained evidence, never tracee memory.
func (p *sourceRecoveryNativePreemption) verifyReplay(tid int, registers syscall.PtraceRegs, original *sourceRecoveryHold) error {
	if !p.enabled {
		return nil
	}
	positive := *original
	matched, err := positive.replay(tid, registers)
	if err != nil || !matched {
		return errors.Join(errors.New("actual native recovery replay proof missing"), err)
	}
	if repeated, err := positive.replay(tid, registers); err != nil || repeated {
		return errors.Join(errors.New("consumed recovery signal proof admitted another replay"), err)
	}
	for index := range 16 {
		changed := *original
		switch index {
		case 0:
			changed.interrupted = false
		case 1:
			changed.entry.registers.R14++
		case 2:
			changed.entry.registers.Rsp++
		case 3:
			changed.entry.registers.Rbp++
		case 4:
			changed.entry.registers.Rax++
		case 5:
			changed.entry.registers.Rbx++
		case 6:
			changed.entry.registers.Rcx++
		case 7:
			changed.entry.registers.Rdi++
		case 8:
			changed.entry.registers.Rsi++
		case 9:
			changed.entry.registers.R8++
		case 10:
			changed.entry.registers.R9++
		case 11:
			changed.entry.registers.R10++
		case 12:
			changed.entry.registers.R11++
		case 13:
			changed.entry.caller[0] ^= 1
		case 14:
			changed.stackArguments[0] ^= 1
		case 15:
			changed.entry.raw = append([]byte(nil), original.entry.raw...)
			if len(changed.entry.raw) == 0 {
				return errors.New("actual recovery journal image absent")
			}
			changed.entry.raw[0] ^= 1
		}
		if admitted, err := changed.replay(tid, registers); err != nil || admitted {
			return errors.Join(fmt.Errorf("native recovery replay evidence substitution %d admitted or unobservable", index), err)
		}
	}
	_, err = fmt.Println("SOURCE_RECOVERY_NATIVE_REPLAY_GUARDS substitutions=16 consumed_signal_refused=true")
	return err
}
