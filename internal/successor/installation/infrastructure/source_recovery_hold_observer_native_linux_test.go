//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const sourceRecoveryHoldRecordSymbol = "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure.(*sourceBirths).record"
const sourceRecoveryHoldNativeAdmissionSymbol = "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure.TestSourceRecoveryCancelNativeDriver"
const sourceRecoveryHoldBody = "actual original born inode repaired by new file owner\n"

// An outside observation declaration, never Source or bootstrap authority.
// Only the two actual original-born frontiers are admitted for cancellation.
type sourceRecoveryHoldProfile struct {
	Phase         string `json:"phase"`
	Path          string `json:"path"`
	BodySHA256    string `json:"body_sha256"`
	Generation    string `json:"generation"`
	BindingSHA256 string `json:"binding_sha256"`
	PrefixRecords int    `json:"prefix_records"`
}

func (p *sourceRecoveryHoldProfile) valid(root, journal string) bool {
	return p != nil && p.Phase == "file-ready" && sourcePath(p.Path) &&
		p.Path == filepath.Join(root, "generations", p.Generation, ProgramName) &&
		sourceCutObserverSHA(p.BodySHA256) && sourceCutObserverSHA(p.Generation) && sourceCutObserverSHA(p.BindingSHA256) &&
		(journal == root+".source-initial-journal" && p.PrefixRecords == 70 ||
			journal == root+".source-replacement-"+p.Generation+"-journal" && p.PrefixRecords == 13)
}

// Holds the actual record entry after real original-inode repair and a durable
// partial envelope. The native fixture supplies no accepting authority; an
// installed observation still requires the new completed exact D6 acceptance.
type sourceRecoveryHold struct {
	journal, payload      string
	lock, operation       string
	profile               sourceRecoveryHoldProfile
	journalPin            string
	acknowledgement       []byte
	born                  sourceRecord
	started               time.Time
	tid                   int
	command               []byte
	signalled             bool
	journalRaw, headerRaw []byte
	payloadRaw            []byte
	journalInfo           os.FileInfo
	headerInfo            os.FileInfo
	payloadInfo           os.FileInfo
}

func newSourceRecoveryHold(scenario, root string, refusal *sourceRefusalObserverProfile) *sourceRecoveryHold {
	if refusal != nil && refusal.RecoveryHold != nil {
		operation := "initial"
		if refusal.RecoveryHold.PrefixRecords == 13 {
			operation = "replacement"
		}
		return &sourceRecoveryHold{journal: refusal.Arguments[7], payload: refusal.RecoveryHold.Path,
			lock: filepath.Join(refusal.Arguments[3], ".ardents-release-decision-lock"), operation: operation,
			profile: *refusal.RecoveryHold, journalPin: refusal.JournalSHA256,
			acknowledgement: []byte("{\"operation\":\"installation.source\",\"outcome\":\"completion-unavailable\"}\n")}
	}
	if scenario != "refusal-recovery-cancel" && scenario != "refusal-recovery-early-notice" {
		return nil
	}
	return &sourceRecoveryHold{journal: filepath.Join(root, "original-journal"), payload: filepath.Join(root, "payload"),
		lock: filepath.Join(root, "release-lock"), operation: "initial",
		profile:         sourceRecoveryHoldProfile{Phase: "file-ready", Path: filepath.Join(root, "payload"), BodySHA256: digest([]byte(sourceRecoveryHoldBody)), Generation: strings.Repeat("a", 64), BindingSHA256: strings.Repeat("b", 64), PrefixRecords: 3},
		acknowledgement: []byte("native-recovery-cancellation-observed\n")}
}

func (h *sourceRecoveryHold) match(tid int, registers *syscall.PtraceRegs) (bool, error) {
	phase, err := sourcePhysicalString(tid, registers.Rbx, registers.Rcx, 128)
	if err != nil {
		return false, err
	}
	if phase != h.profile.Phase {
		return false, nil
	}
	path, err := sourcePhysicalString(tid, registers.Rdi, registers.Rsi, 4096)
	return path == h.payload, err
}

// capture checks the native interrupted image without emitting a cancellation
// event or assuming that its writer is still alive. Physical root-cut observers
// reuse this image check before SIGKILL and after the original root is reaped.
func (h *sourceRecoveryHold) capture() error {
	var err error
	h.journalRaw, h.journalInfo, err = readSourceCutObserverFile(h.journal, 4<<20)
	if err != nil {
		return err
	}
	records, err := decodeSourceRecoveryPrefix(h.journalRaw, sourceRecoverableInitialRecords)
	if err != nil || len(records) != h.profile.PrefixRecords || h.journalPin != "" && digest(h.journalRaw) != h.journalPin {
		return errors.Join(errors.New("actual pinned original prefix required for recovery hold"), err)
	}
	h.born = records[len(records)-1]
	intent, expected := records[len(records)-2], h.born
	expected.Sequence, expected.Phase, expected.Device, expected.Inode = intent.Sequence, "file-intent", 0, 0
	if h.born.Phase != "file-born" || h.born.Path != h.payload || h.born.SHA256 != h.profile.BodySHA256 || intent != expected || h.born.Sequence != intent.Sequence+1 {
		return errors.New("exact last original intent/born pair required for recovery hold")
	}
	h.headerRaw, h.headerInfo, err = readSourceCutObserverFile(h.journal+".recovery-completion", 4<<20)
	if err != nil {
		return err
	}
	var metadata sourceRecoveryCompletion
	ending := []byte(`],"transcript_sha256":""}`)
	if !bytes.HasSuffix(h.headerRaw, []byte(`"records":[`)) || json.Unmarshal(append(bytes.Clone(h.headerRaw), ending...), &metadata) != nil {
		return errors.New("actual new incomplete recovery header required")
	}
	canonical, err := json.Marshal(metadata)
	if err != nil || !bytes.Equal(append(bytes.Clone(h.headerRaw), ending...), canonical) {
		return errors.Join(errors.New("canonical actual incomplete recovery header required"), err)
	}
	native := h.journalInfo.Sys().(*syscall.Stat_t)
	if metadata.Schema != "ardents-source-recovery-completion-v1" || metadata.Operation != h.operation || metadata.Journal != h.journal || metadata.Device != uint64(native.Dev) || metadata.Inode != native.Ino || metadata.PrefixSHA256 != digest(h.journalRaw) || metadata.PrefixRecords != h.profile.PrefixRecords || metadata.Generation != h.profile.Generation || metadata.BindingSHA256 != h.profile.BindingSHA256 {
		return errors.New("recovery header does not bind the pinned original journal")
	}
	h.payloadRaw, h.payloadInfo, err = readSourceRecoveryHeldPayload(h.payload, h.born)
	if err != nil || digest(h.payloadRaw) != h.profile.BodySHA256 {
		return errors.Join(errors.New("actual repaired original payload required"), err)
	}
	born := h.payloadInfo.Sys().(*syscall.Stat_t)
	if uint64(born.Dev) != h.born.Device || born.Ino != h.born.Inode {
		return errors.New("recovery hold lost original born inode")
	}
	return nil
}

func (h *sourceRecoveryHold) freeze(tid int, result *sourceCutObservation) error {
	if err := h.capture(); err != nil {
		return err
	}
	for _, path := range []string{h.journal, h.journal + ".recovery-completion", h.lock} {
		if err := probeSourceRefusalLease(path, "", 0600, true); err != nil {
			return err
		}
	}
	h.tid, result.HeldRootThread, result.RecoveryHeldBeforeRecord = tid, tid, true
	result.RecoveryRootOnlyAtHold = true
	result.SelectedJournalSHA256 = digest(h.journalRaw)
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = fmt.Printf("SOURCE_RECOVERY_HOLD_READY %s\n", raw)
	return err
}

func (h *sourceRecoveryHold) preserved() error {
	for _, original := range []struct {
		path string
		raw  []byte
		info os.FileInfo
	}{{h.journal, h.journalRaw, h.journalInfo}, {h.journal + ".recovery-completion", h.headerRaw, h.headerInfo}, {h.payload, h.payloadRaw, h.payloadInfo}} {
		var raw []byte
		var info os.FileInfo
		var err error
		if original.path == h.payload {
			raw, info, err = readSourceRecoveryHeldPayload(original.path, h.born)
		} else {
			raw, info, err = readSourceCutObserverFile(original.path, 4<<20)
		}
		if err != nil || !bytes.Equal(raw, original.raw) || !sameSourceFile(info, original.info) || info.Mode() != original.info.Mode() {
			return errors.Join(errors.New("held original recovery image changed"), err)
		}
	}
	return nil
}

// The original born record pins access and inode before actual repair. Unlike
// journals, its recorded final access may be0444/0555 or an account-owned0600.
// Observe exactly that access; never chmod it to make the observer succeed.
func readSourceRecoveryHeldPayload(path string, born sourceRecord) (raw []byte, info os.FileInfo, returned error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	defer func() { returned = errors.Join(returned, f.Close()) }()
	info, err = f.Stat()
	if err != nil || !sourceAccess(info, os.FileMode(born.Mode), born.UID, born.GID, false) || info.Size() <= 0 || info.Size() > 64<<20 {
		return nil, nil, errors.Join(errors.New("actual bounded recorded-access payload required"), err)
	}
	native := info.Sys().(*syscall.Stat_t)
	var fs syscall.Statfs_t
	if uint64(native.Dev) != born.Device || native.Ino != born.Inode || syscall.Fstatfs(int(f.Fd()), &fs) != nil || fs.Type != 0xef53 {
		return nil, nil, errors.New("actual original ext4 born inode required")
	}
	raw, err = io.ReadAll(io.LimitReader(f, (64<<20)+1))
	after, statErr := f.Stat()
	named, namedErr := os.Lstat(path)
	if err != nil || statErr != nil || namedErr != nil || int64(len(raw)) != info.Size() || !sameSourceFile(info, after) || !sameSourceFile(info, named) {
		return nil, nil, errors.Join(errors.New("held original payload changed during observation"), err, statErr, namedErr)
	}
	return raw, info, nil
}

func TestSourceRecoveryHeldPayloadNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("actual native file stand required", err)
	}
	for _, mode := range []os.FileMode{0444, 0555, 0600} {
		path := filepath.Join(t.TempDir(), "actual-born-payload")
		body := bytes.Repeat([]byte("a"), 8193)
		if err := os.WriteFile(path, body, mode); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		native := info.Sys().(*syscall.Stat_t)
		born := sourceRecord{Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: uint32(mode)}
		raw, actual, err := readSourceRecoveryHeldPayload(path, born)
		if err != nil || !bytes.Equal(raw, body) || !sameSourceFile(info, actual) {
			t.Fatal("actual recorded-access payload observation", mode, err)
		}
		if err := os.Chmod(path, 0640); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readSourceRecoveryHeldPayload(path, born); err == nil {
			t.Fatal("changed recorded access accepted")
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		born.Inode++
		if _, _, err := readSourceRecoveryHeldPayload(path, born); err == nil {
			t.Fatal("foreign original inode accepted")
		}
	}
}

func (h *sourceRecoveryHold) control(pid int, groups map[int]int, tracked map[int]bool, output []byte, result *sourceCutObservation) error {
	// Process birth precedes the canonical operation's two-minute context.
	// This conservative monotonic observation bound neither extends that
	// context nor counts a previously processed timeout as our cancellation.
	if h.started.IsZero() || time.Since(h.started) >= 2*time.Minute {
		return errors.New("processed recovery cancellation did not precede original two-minute bound")
	}
	if !h.signalled {
		if bytes.Contains(output, h.acknowledgement) {
			return errors.New("processed cancellation notice preceded outside signal")
		}
		var raw [8]byte
		n, err := syscall.Read(int(os.Stdin.Fd()), raw[:])
		if err != nil && err != syscall.EAGAIN && err != syscall.EWOULDBLOCK && err != syscall.EINTR {
			return err
		}
		if n == 0 && err == nil {
			return errors.New("outside recovery control closed before cancellation")
		}
		if n > 0 {
			h.command = append(h.command, raw[:n]...)
			if !bytes.HasPrefix([]byte("cancel\n"), h.command) {
				return errors.New("exact outside recovery cancel required")
			}
		}
		if string(h.command) != "cancel\n" {
			return nil
		}
		if err := h.preserved(); err != nil {
			return err
		}
		for tid, fresh := range tracked {
			if tid != h.tid && !fresh && groups[tid] == pid {
				if err := syscall.Tgkill(pid, tid, syscall.SIGTERM); err != nil {
					return err
				}
				h.signalled, result.SignalledRootThread = true, tid
				break
			}
		}
		if !h.signalled {
			return errors.New("original distinct recovery signal thread missing")
		}
	}
	if !bytes.Contains(output, h.acknowledgement) {
		return nil
	}
	if err := h.preserved(); err != nil {
		return err
	}
	result.RecoveryCancellationElapsedNS = time.Since(h.started).Nanoseconds()
	if result.RecoveryCancellationElapsedNS >= int64(2*time.Minute) {
		return errors.New("processed recovery cancellation exceeded original two-minute bound")
	}
	var registers syscall.PtraceRegs
	if err := syscall.PtraceGetRegs(h.tid, &registers); err != nil || registers.Rip != result.RecoveryHoldEntry {
		return errors.Join(errors.New("original recovery hold entry moved"), err)
	}
	if err := sourceCutPtrace(syscall.PTRACE_POKEUSR, h.tid, sourceCutDebugRegisters+7*8, 1); err != nil {
		return err
	}
	if err := resumeSourceCutBreakpoint(h.tid, &registers); err != nil {
		return err
	}
	h.tid, result.ExternalCancellationAcknowledged = 0, true
	return nil
}

const sourceRecoveryNativeHelperCount = 40

func joinSourceRecoveryNativeHelpers(t *testing.T) {
	t.Helper()
	for range sourceRecoveryNativeHelperCount {
		child := exec.Command("/bin/true")
		if err := child.Run(); err != nil {
			t.Fatal("original native helper join", err)
		}
		fmt.Printf("SOURCE_RECOVERY_NATIVE_JOINED_CHILD pid=%d\n", child.Process.Pid)
	}
}

// Real prefix/writer/inode/lease cancellation only. These fixture hashes supply
// no Release, semantic history, account, native manager or Source authority.
func TestSourceRecoveryCancelNativeDriver(t *testing.T) {
	scenario := os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_CASE")
	if scenario != "refusal-recovery-cancel" && scenario != "refusal-recovery-early-notice" {
		return
	}
	root := os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_ROOT")
	if !sourcePath(root) || os.Geteuid() != 0 || os.Getenv("GOGC") != "off" || os.Getenv("GOMEMLIMIT") != "off" {
		t.Fatal("original no-GC native recovery driver required")
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(root, "release-lock"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	done, stop := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-signals:
			cancel()
			fmt.Println("native-recovery-cancellation-observed")
		case <-stop:
		}
	}()
	stopSignals := sync.OnceFunc(func() { signal.Stop(signals); close(stop); <-done })
	defer stopSignals()
	// Admission precedes these real, separately reaped helper lifetimes. The
	// installed verifier also runs native helpers before reaching its record.
	// Their historical existence must not be confused with a live descendant.
	joinSourceRecoveryNativeHelpers(t)
	liveChildren := 0
	switch os.Getenv("ARDENTS_SOURCE_RECOVERY_NATIVE_LIVE_CHILD") {
	case "":
	case "1":
		liveChildren = 1
	case "32":
		liveChildren = 32
	default:
		t.Fatal("closed native live-child fixture count required")
	}
	for range liveChildren {
		child := exec.Command("/bin/sleep", "60")
		if err := child.Start(); err != nil {
			t.Fatal("original live-child negative fixture", err)
		}
		defer func() {
			killed, waited := child.Process.Kill(), child.Wait()
			if killed != nil || waited == nil {
				t.Error("live-child fixture cleanup did not retain interruption", killed, waited)
			}
		}()
		fmt.Printf("SOURCE_RECOVERY_NATIVE_LIVE_CHILD pid=%d\n", child.Process.Pid)
	}
	if scenario == "refusal-recovery-early-notice" {
		if _, err := fmt.Println("native-recovery-cancellation-observed"); err != nil {
			t.Fatal(err)
		}
	}
	journal, payload := filepath.Join(root, "original-journal"), filepath.Join(root, "payload")
	original, err := openSourceRecoverableInitialBirths(ctx, journal, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer original.close()
	hash := digest([]byte(sourceRecoveryHoldBody))
	if err := original.record("file-intent", payload, nil, hash, 0600, 0, 0); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(payload, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	info, statErr := f.Stat()
	if err := errors.Join(statErr, f.Sync(), f.Close(), syncSourceParent(payload)); err != nil {
		t.Fatal(err)
	}
	if err := original.record("file-born", payload, info, hash, 0600, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := original.close(); err != nil {
		t.Fatal(err)
	}
	prefix, err := openSourceRecoveryPrefix(ctx, journal, "initial")
	if err != nil {
		t.Fatal(err)
	}
	defer prefix.close()
	writer, err := openSourceRecoveryBirths(prefix, strings.Repeat("a", 64), strings.Repeat("b", 64), map[string]bool{payload: true})
	defer writer.close()
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.resumeSourceBornFile([]byte(sourceRecoveryHoldBody), prefix.check); !errors.Is(err, context.Canceled) || !sourceJoinedCancellation(err) {
		t.Fatal("actual recovery record did not retain cancellation", err)
	}
	fmt.Println("native-recovery-record-refused")
	if err := writer.close(); !errors.Is(err, context.Canceled) || !sourceJoinedCancellation(err) {
		t.Fatal("actual recovery writer Close lost cancellation", err)
	}
	fmt.Println("native-recovery-writer-closed")
	if err := prefix.close(); !errors.Is(err, context.Canceled) || !sourceJoinedCancellation(err) {
		t.Fatal("original prefix Close lost cancellation", err)
	}
	fmt.Println("native-recovery-prefix-closed")
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	stopSignals()
	fmt.Println("native-recovery-signal-user-joined")
	if err := sourceRefusalNativeReturn(); err != nil {
		t.Fatal(err)
	}
}
