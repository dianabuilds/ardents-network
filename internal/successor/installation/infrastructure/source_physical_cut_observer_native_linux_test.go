//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// A closed native file/process control, never an installed profile. Function
// entry breakpoints precede the actual I/O or durable record, without changing
// instructions, arguments or program state. These windows cannot publish the
// eligible ACK-cut proof and cannot authorize accepting Source recovery.
type sourcePhysicalCut struct {
	window, symbol, lastPhase, nextPhase, body string
	records                                    int
	absent                                     bool
	root                                       bool
	killRoot                                   bool
	externalCut                                bool
	canonical                                  *sourceCanonicalPhysicalProfile
	recovery                                   *sourceRecoveryPhysicalProfile
	startup                                    *sourceStartupPhysicalProfile
	retirement                                 *sourceRetirementPhysicalProfile
	publicationBefore                          *sourcePhysicalPublicationPreimage
}

func sourcePhysicalWindow(window string) *sourcePhysicalCut {
	const owner = "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure.(*sourceBirths)."
	switch window {
	case "journal-before-sync":
		return &sourcePhysicalCut{window: window, symbol: owner + "sync", lastPhase: "admitted", records: 2, absent: true}
	case "file-born-before-record":
		return &sourcePhysicalCut{window: window, symbol: owner + "record", lastPhase: "file-intent", nextPhase: "file-born", records: 3}
	case "file-ready-before-record":
		return &sourcePhysicalCut{window: window, symbol: owner + "record", lastPhase: "file-born", nextPhase: "file-ready", body: "original next effect\n", records: 4}
	case "file-complete-before-close":
		return &sourcePhysicalCut{window: window, symbol: owner + "close", lastPhase: "file-ready", body: "original next effect\n", records: 5}
	case "pending-before-unlink":
		return &sourcePhysicalCut{window: window, symbol: "os.Remove", lastPhase: "file-ready", body: "original next effect\n", records: 5}
	case "root-journal-before-sync", "root-file-born-before-record", "root-file-ready-before-record", "root-file-complete-before-close":
		plan := sourcePhysicalWindow(strings.TrimPrefix(window, "root-"))
		plan.window, plan.root = window, true
		return plan
	case "root-process-journal-before-sync", "root-process-file-born-before-record", "root-process-file-ready-before-record", "root-process-file-complete-before-close":
		plan := sourcePhysicalWindow(strings.TrimPrefix(window, "root-process-"))
		plan.window, plan.root, plan.killRoot = window, true, true
		return plan
	case "root-controlled-file-ready-before-record":
		plan := sourcePhysicalWindow("root-process-file-ready-before-record")
		plan.window, plan.externalCut = window, true
		return plan
	default:
		return nil
	}
}

func (p *sourcePhysicalCut) valid() bool {
	if p.retirement != nil {
		exact := sourceRetirementPhysicalWindow(p.retirement)
		return exact != nil && *p == *exact
	}
	if p.startup != nil {
		exact := sourceStartupPhysicalWindow(p.startup)
		return exact != nil && *p == *exact
	}
	if p.recovery != nil {
		exact := sourceRecoveryPhysicalWindow(p.recovery)
		return exact != nil && *p == *exact
	}
	if p.canonical != nil {
		exact := sourceCanonicalPhysicalWindow(p.canonical)
		return exact != nil && *p == *exact
	}
	exact := sourcePhysicalWindow(p.window)
	return exact != nil && *p == *exact
}

type sourcePhysicalImage struct {
	journalInfo, payloadInfo os.FileInfo
	journalSHA, payloadSHA   string
	pendingInfo              os.FileInfo
	pendingSHA               string
}

func sourcePhysicalSameProgram(group int, original os.FileInfo) (bool, error) {
	actual, err := os.Stat("/proc/" + strconv.Itoa(group) + "/exe")
	return err == nil && os.SameFile(original, actual), err
}

// The selected Go1.27.3 toolchain's src/cmd/compile/abi-internal.md assigns the
// receiver to RAX and string arguments to RBX/RCX and RDI/RSI on amd64. The
// observer separately checks that exact build version before using these reads.
func sourcePhysicalString(tid int, address, size uint64, maximum int) (string, error) {
	if address == 0 || size == 0 || size > uint64(maximum) {
		return "", errors.New("bounded actual entry string required")
	}
	raw := make([]byte, int(size))
	n, err := syscall.PtracePeekData(tid, uintptr(address), raw)
	if err != nil || n != len(raw) {
		return "", errors.Join(io.ErrUnexpectedEOF, err)
	}
	return string(raw), nil
}

// Read one bounded native owner word without changing tracee state.
func sourcePhysicalWord(tid int, base uint64, offset uintptr) (uint64, error) {
	if base == 0 || uint64(offset) > 4096 || base+uint64(offset) < base {
		return 0, errors.New("bounded original owner address required")
	}
	var raw [8]byte
	n, err := syscall.PtracePeekData(tid, uintptr(base+uint64(offset)), raw[:])
	if err != nil || n != len(raw) {
		return 0, errors.Join(errors.New("actual original owner could not be read"), err)
	}
	return binary.LittleEndian.Uint64(raw[:]), nil
}

// The exact Go1.27.3 linux/amd64 os.File argument contains *os.file and its
// poll.FD.Sysfd. Derive offsets from the same toolchain's real types, without
// unsafe or writes to the tracee. Bind that live descriptor to the independently
// opened journal, rather than infer the Sync target from visible bytes alone.
func sourcePhysicalJournalArgument(tid int, address uint64, journal os.FileInfo) error {
	file, ok := reflect.TypeFor[os.File]().FieldByName("file")
	if !ok || file.Type.Kind() != reflect.Pointer || file.Type.Elem().Kind() != reflect.Struct {
		return errors.New("exact native os.File representation required")
	}
	poll, ok := file.Type.Elem().FieldByName("pfd")
	if !ok || poll.Type.Kind() != reflect.Struct {
		return errors.New("exact native os.file poll descriptor required")
	}
	fd, ok := poll.Type.FieldByName("Sysfd")
	if !ok || fd.Type.Kind() != reflect.Int || fd.Type.Size() != 8 {
		return errors.New("exact native poll Sysfd representation required")
	}
	inner, err := sourcePhysicalWord(tid, address, file.Offset)
	if err != nil {
		return err
	}
	number, err := sourcePhysicalWord(tid, inner, poll.Offset+fd.Offset)
	if err != nil || number > 1<<20 {
		return errors.Join(errors.New("bounded live original file descriptor required"), err)
	}
	actual, err := os.Stat("/proc/" + strconv.Itoa(tid) + "/fd/" + strconv.FormatUint(number, 10))
	if err != nil || journal == nil || !os.SameFile(journal, actual) {
		return errors.Join(errors.New("actual Sync argument is not the original journal descriptor"), err)
	}
	return nil
}

// A complete-looking journal or a reader's Close is not the original writer's
// first Close. Read the actual method receiver and bind its still-live journal
// descriptor; the exact same-source Go ABI guard precedes these observations.
func sourcePhysicalBirthsCloseReceiver(tid int, address uint64, journal os.FileInfo) error {
	return sourcePhysicalLiveWriterReceiver(tid, address, journal, reflect.TypeFor[sourceBirths]())
}

// Both native journal owners retain a closed bit and their own sourceBirth
// descriptor. The same-source Go ABI guard precedes every actual memory read.
func sourcePhysicalLiveWriterReceiver(tid int, address uint64, journal os.FileInfo, owner reflect.Type) error {
	closed, ok := owner.FieldByName("closed")
	if !ok || closed.Type.Kind() != reflect.Bool || address == 0 || closed.Offset > 4096 || address+uint64(closed.Offset) < address {
		return errors.New("exact original writer receiver required")
	}
	var state [1]byte
	n, err := syscall.PtracePeekData(tid, uintptr(address+uint64(closed.Offset)), state[:])
	if err != nil || n != 1 || state[0] != 0 {
		return errors.Join(errors.New("original writer receiver is already closed"), err)
	}
	birth, ok := owner.FieldByName("journal")
	if !ok || birth.Type != reflect.TypeFor[*sourceBirth]() {
		return errors.New("exact original journal owner required")
	}
	file, ok := reflect.TypeFor[sourceBirth]().FieldByName("file")
	if !ok || file.Type != reflect.TypeFor[*os.File]() {
		return errors.New("exact original journal file required")
	}
	original, err := sourcePhysicalWord(tid, address, birth.Offset)
	if err != nil {
		return err
	}
	descriptor, err := sourcePhysicalWord(tid, original, file.Offset)
	if err != nil {
		return err
	}
	return sourcePhysicalJournalArgument(tid, descriptor, journal)
}

func readSourcePhysicalPayload(path string) (raw []byte, info os.FileInfo, returned error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	defer func() { returned = errors.Join(returned, f.Close()) }()
	info, err = f.Stat()
	if err != nil || !sourceAccess(info, 0600, 0, 0, false) || info.Size() < 0 || info.Size() > 4096 {
		return nil, nil, errors.Join(errors.New("bounded original root0600 payload required"), err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(f.Fd()), &fs); err != nil || fs.Type != 0xef53 {
		return nil, nil, errors.Join(errors.New("original ext4 payload required"), err)
	}
	raw, err = io.ReadAll(io.LimitReader(f, 4097))
	after, statErr := f.Stat()
	named, namedErr := os.Lstat(path)
	if err != nil || statErr != nil || namedErr != nil || int64(len(raw)) != info.Size() || !sameSourceFile(info, after) || !sameSourceFile(info, named) || info.Mode() != after.Mode() || info.Size() != after.Size() {
		return nil, nil, errors.Join(errors.New("original payload changed during read"), err, statErr, namedErr)
	}
	return raw, info, nil
}

func sourcePhysicalSHA(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (p *sourcePhysicalCut) match(tid int, registers *syscall.PtraceRegs, journal, root string) (bool, *sourcePhysicalImage, error) {
	if p.retirement != nil {
		return p.matchRetirement(tid, registers, journal, root)
	}
	if p.startup != nil {
		return p.matchStartup(tid, registers, journal, root)
	}
	if p.recovery != nil {
		return p.matchRecovery(tid, registers, journal, root)
	}
	if p.canonical != nil {
		return p.matchCanonical(tid, registers, journal, root)
	}
	info, err := os.Lstat(journal)
	if errors.Is(err, os.ErrNotExist) || err == nil && info.Size() == 0 {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	raw, info, err := readSourceCutObserverFile(journal, 4<<20)
	if err != nil {
		return false, nil, err
	}
	if bytes.Count(raw, []byte{'\n'}) != p.records {
		return false, nil, nil
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	var last sourceRecord
	if json.Unmarshal(lines[len(lines)-1], &last) != nil || last.Sequence != p.records-1 || last.Phase != p.lastPhase {
		return false, nil, errors.New("physical frontier record differs")
	}
	path := filepath.Join(root, "next-effect")
	if p.window == "pending-before-unlink" {
		actual, err := sourcePhysicalString(tid, registers.Rax, registers.Rbx, 4096)
		if err != nil || actual != path {
			return false, nil, err
		}
	}
	if p.nextPhase != "" {
		phase, err := sourcePhysicalString(tid, registers.Rbx, registers.Rcx, 64)
		if err != nil {
			return false, nil, err
		}
		actualPath, err := sourcePhysicalString(tid, registers.Rdi, registers.Rsi, 4096)
		if err != nil {
			return false, nil, err
		}
		if phase != p.nextPhase || actualPath != path {
			return false, nil, errors.New("actual physical record arguments differ")
		}
	}
	image := &sourcePhysicalImage{journalInfo: info, journalSHA: sourcePhysicalSHA(raw)}
	if strings.HasSuffix(p.window, "file-complete-before-close") {
		if err := sourcePhysicalBirthsCloseReceiver(tid, registers.Rax, info); err != nil {
			return false, nil, err
		}
		foreign, err := os.Stat("/proc/" + strconv.Itoa(tid) + "/exe")
		if err != nil || sourcePhysicalBirthsCloseReceiver(tid, registers.Rax, foreign) == nil || sourcePhysicalBirthsCloseReceiver(tid, 0, info) == nil {
			return false, nil, errors.Join(errors.New("foreign or nil original Close receiver accepted"), err)
		}
		if verifySourcePhysicalRootFDs(tid, image) == nil {
			return false, nil, errors.New("live original writer was reported descriptor-free")
		}
	}
	if p.absent {
		if err := sourcePhysicalJournalArgument(tid, registers.Rbx, info); err != nil {
			return false, nil, err
		}
		foreign, err := os.Stat("/proc/" + strconv.Itoa(tid) + "/exe")
		if err != nil || sourcePhysicalJournalArgument(tid, registers.Rbx, foreign) == nil {
			return false, nil, errors.Join(errors.New("foreign native inode accepted as Sync journal argument"), err)
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return false, nil, errors.Join(errors.New("pre-sync performed next effect"), err)
		}
	} else {
		body, fileInfo, err := readSourcePhysicalPayload(path)
		if err != nil || string(body) != p.body {
			return false, nil, errors.Join(errors.New("actual physical payload differs"), err)
		}
		image.payloadInfo, image.payloadSHA = fileInfo, sourcePhysicalSHA(body)
	}
	if p.window == "pending-before-unlink" {
		if err := errors.Join(verifySourcePhysicalRootFDs(tid, image), probeSourceRefusalLease(journal, image.journalSHA, 0600, false)); err != nil {
			return false, nil, err
		}
	}
	return true, image, nil
}

func (p *sourcePhysicalCut) verifyRetained(journal, root string, before *sourcePhysicalImage) error {
	if p.retirement != nil {
		return p.verifyRetirement(before)
	}
	if p.startup != nil {
		return p.verifyStartup(before)
	}
	if p.recovery != nil {
		return p.verifyRecovery(before)
	}
	if p.canonical != nil {
		return p.verifyCanonicalRetained(journal, root, before)
	}
	raw, after, err := readSourceCutObserverFile(journal, 4<<20)
	if err != nil || before == nil || !sameSourceFile(before.journalInfo, after) || sourcePhysicalSHA(raw) != before.journalSHA {
		return errors.Join(errors.New("unsafe original prefix changed after join"), err)
	}
	path := filepath.Join(root, "next-effect")
	if p.absent {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(errors.New("cancelled unsafe cut performed next effect"), err)
		}
	} else {
		body, info, err := readSourcePhysicalPayload(path)
		if err != nil || !sameSourceFile(before.payloadInfo, info) || sourcePhysicalSHA(body) != before.payloadSHA {
			return errors.Join(errors.New("unsafe original payload changed after join"), err)
		}
	}
	for _, suffix := range []string{".original-cut-journal", ".original-cut-result", ".original-cut-pending", ".recovery-completion"} {
		if _, err := os.Lstat(journal + suffix); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(errors.New("unsafe cut manufactured acceptance evidence"), err)
		}
	}
	// Independently held original exclusion after the real root/controller join
	// still cannot manufacture eligible Source provenance. This fixture has only
	// native record framing, deliberately not an accepting Source grammar.
	fresh, err := openSourceRecoveryPrefix(context.Background(), journal, "initial")
	if err != nil {
		return err
	}
	if !sameSourceFile(before.journalInfo, fresh.journal.info) || !bytes.Equal(raw, fresh.journal.raw) {
		return errors.Join(errors.New("fresh unsafe prefix differs"), fresh.close())
	}
	missing := fresh.readOriginalCut(strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64))
	closed := fresh.close()
	if !sourcePhysicalRefusalClosed(missing, closed) {
		return errors.Join(errors.New("unsafe cut granted provenance or lost original failure"), missing, closed)
	}
	return nil
}

func TestSourcePhysicalCutObserverNative(t *testing.T) {
	window := os.Getenv("ARDENTS_SOURCE_PHYSICAL_OBSERVER_WINDOW")
	if window == "" {
		return
	}
	plan := sourcePhysicalWindow(window)
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if plan == nil || err != nil || string(marker) != "ardents-source-stopped-native-v1\n" {
		t.Fatal("closed native physical stand required", err)
	}
	scenario := "unsafe-external-cut"
	if plan.root {
		scenario = "unsafe-root-cut"
	}
	result, err := observeSourceCut(os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM"), os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_ROOT"), os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT"), scenario, nil, false, 2, plan)
	raw, encoded := json.Marshal(result)
	fmt.Printf("SOURCE_PHYSICAL_CUT_OBSERVATION %s\n", raw)
	if err != nil || encoded != nil {
		t.Fatal("actual native unsafe physical cut", err, encoded)
	}
}

// The canonical pending-completion diagnostic exists only while original work
// is still unjoined. A native driver that always prints it in a return defer
// masks fast coordinator kill/join. Exercise the real exit-stop hold instead.
func TestSourcePhysicalCoordinatorJoinNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("original native file/process stand required", err)
	}
	root, err := os.MkdirTemp(os.Getenv("ARDENTS_SOURCE_STOPPED_FUTURE_PARENT"), "coordinator-join-")
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), program, "-test.run=^TestSourcePhysicalCutObserverNative$", "-test.v", "-test.timeout=35s")
	command.Env = append(os.Environ(), "GOMAXPROCS=2", "ARDENTS_SOURCE_CUT_NATIVE_DELAYED_NOTICE=1",
		"ARDENTS_SOURCE_PHYSICAL_OBSERVER_WINDOW=file-born-before-record", "ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM="+program,
		"ARDENTS_SOURCE_CUT_OBSERVER_ROOT="+root, "ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT="+filepath.Join(root, "original.log"))
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("actual original coordinator join hold: %v\n%s", err, output)
	}
	var report sourceCutObservation
	reports := 0
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		if raw, ok := bytes.CutPrefix(line, []byte("SOURCE_PHYSICAL_CUT_OBSERVATION ")); ok {
			reports++
			if err := json.Unmarshal(raw, &report); err != nil {
				t.Fatal(err)
			}
		}
	}
	if reports != 1 || report.InstalledExecution || report.InspectionOnly || !report.CoordinatorExitHeldBeforeJoin || !report.CoordinatorExitReleasedAfterNotice ||
		!report.ExternalCancellationAcknowledged || !report.HeldCoordinatorKilled || report.RootExit != 0 || report.SelectedRecords != 3 || report.Processes != 2 || report.ThreadsJoined < 2 || report.OriginalCutSHA256 != "" {
		t.Fatalf("actual native pending-notice/original join proof missing: %+v", report)
	}
	t.Logf("retained native original join proof %s\n%s", root, output)
}
