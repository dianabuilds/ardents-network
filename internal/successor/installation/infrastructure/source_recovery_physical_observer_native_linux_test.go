//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// An outside process-cut declaration, never original-cut or recovery authority.
// The actual canonical recovery must pass its own admission before these entries.
type sourceRecoveryPhysicalProfile struct {
	sourceCutObserverProfile
	Window            string `json:"window"`
	Generation        string `json:"generation"`
	BindingSHA256     string `json:"binding_sha256"`
	JournalSHA256     string `json:"journal_sha256"`
	OriginalCutSHA256 string `json:"original_cut_sha256"`
	root              string
}

func readSourceRecoveryPhysicalProfile(path, pin, program, root string) (*sourceRecoveryPhysicalProfile, error) {
	if !sourcePath(path) || !sourcePath(root) || !sourceCutObserverSHA(pin) {
		return nil, errors.New("exact root recovery profile path/pin required")
	}
	raw, _, err := readSourceCutObserverFile(path, 64<<10)
	if err != nil || sourcePhysicalSHA(raw) != pin {
		return nil, errors.Join(errors.New("root recovery declaration pin differs"), err)
	}
	p := new(sourceRecoveryPhysicalProfile)
	if err := json.Unmarshal(raw, p); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return nil, errors.Join(errors.New("one exact canonical recovery process-cut declaration required"), err)
	}
	p.sha256, p.root = pin, root
	if program != filepath.Join(p.argument(2), ProgramName) || sourceRecoveryPhysicalWindow(p) == nil {
		return nil, errors.New("finite original root recovery command required")
	}
	return p, nil
}

func (p *sourceRecoveryPhysicalProfile) argument(index int) string {
	if p == nil || index < 0 || index >= len(p.Arguments) {
		return ""
	}
	return p.Arguments[index]
}

func (p *sourceRecoveryPhysicalProfile) initial() bool {
	return p.Journal == p.root+".source-initial-journal"
}

func sourceRecoveryPhysicalWindow(p *sourceRecoveryPhysicalProfile) *sourcePhysicalCut {
	if p == nil || p.Schema != "source-recovery-physical-cut-observer-v1" || !sourcePath(p.root) || len(p.Arguments) != 8 ||
		p.Arguments[0] != "installation" || p.Arguments[1] != "recover-source" || p.Arguments[4] != "alpha" || !sourceCutObserverNetwork(p.Arguments[5]) ||
		!sourceCutObserverSHA(p.sha256) || !sourceCutObserverSHA(p.ProgramSHA256) || !sourceCutObserverSHA(p.PublicReceiptSHA256) ||
		!sourceCutObserverSHA(p.Generation) || !sourceCutObserverSHA(p.BindingSHA256) || !sourceCutObserverSHA(p.JournalSHA256) || !sourceCutObserverSHA(p.OriginalCutSHA256) {
		return nil
	}
	for _, index := range []int{2, 3, 6, 7} {
		if !sourcePath(p.Arguments[index]) {
			return nil
		}
	}
	if p.Arguments[7] != p.Journal || !p.initial() && p.Journal != p.root+".source-replacement-"+p.Generation+"-journal" {
		return nil
	}
	born, full, complete := 13, 55, "replacement-verified"
	if p.initial() {
		born, full, complete = 70, 115, "stopped-verified"
	}
	if p.PrefixRecords != born && p.PrefixRecords != full || p.PrefixRecords == born && p.LastPhase != "file-born" || p.PrefixRecords == full && p.LastPhase != complete {
		return nil
	}
	plan := &sourcePhysicalCut{window: p.Window, root: true, killRoot: true, recovery: p, records: p.PrefixRecords, lastPhase: p.LastPhase}
	switch p.Window {
	case "repaired-body-before-record":
		if p.PrefixRecords != born {
			return nil
		}
		plan.symbol = sourceRecoveryHoldRecordSymbol
	case "completion-before-close":
		plan.symbol = "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure.(*sourceBirths).close"
	case "pending-before-unlink":
		plan.symbol = "os.Remove"
	default:
		return nil
	}
	return plan
}

func (p *sourceRecoveryPhysicalProfile) pending() (string, string) {
	if p.initial() {
		return sourceInitialPendingPath(p.root), "ardents-source-initial-pending-v1"
	}
	return filepath.Join(p.root, "source-replacement-pending"), "ardents-source-replacement-pending-v1"
}

func (p *sourceRecoveryPhysicalProfile) repairedHold() *sourceRecoveryHold {
	operation := "replacement"
	if p.initial() {
		operation = "initial"
	}
	return &sourceRecoveryHold{journal: p.Journal, journalPin: p.JournalSHA256, operation: operation,
		payload: filepath.Join(p.root, "generations", p.Generation, ProgramName),
		profile: sourceRecoveryHoldProfile{Phase: "file-ready", Path: filepath.Join(p.root, "generations", p.Generation, ProgramName),
			BodySHA256: p.ProgramSHA256, Generation: p.Generation, BindingSHA256: p.BindingSHA256, PrefixRecords: p.PrefixRecords}}
}

// The first repaired file is complete on its recorded original inode while
// the separate envelope still has zero suffix records. No sealed candidate or
// completed publication exists at this window, so their later image cannot
// substitute for this deliberately incomplete one.
func (p *sourceRecoveryPhysicalProfile) repairedImage() (*sourcePhysicalImage, error) {
	hold := p.repairedHold()
	if err := hold.capture(); err != nil {
		return nil, err
	}
	records, err := decodeSourceRecoveryPrefix(hold.journalRaw, sourceRecoverableInitialRecords)
	first := "initial"
	if p.initial() {
		first = sourceRecoverableInitialPhase
	}
	if err != nil || records[0].Phase != first || records[len(records)-1].Phase != p.LastPhase {
		return nil, errors.Join(errors.New("original repaired-file producer differs"), err)
	}
	pendingPath, schema := p.pending()
	pending, pendingInfo, err := readSourceCutObserverFile(pendingPath, 4096)
	if err != nil || !bytes.Equal(pending, []byte(schema+"\n"+p.BindingSHA256+"\n")) {
		return nil, errors.Join(errors.New("original repaired-file pending differs"), err)
	}
	native := pendingInfo.Sys().(*syscall.Stat_t)
	if err := sourcePhysicalBirthTriple(records, sourceRecord{Path: pendingPath, Device: uint64(native.Dev), Inode: native.Ino, SHA256: sourcePhysicalSHA(pending), Mode: 0600}, "file"); err != nil {
		return nil, err
	}
	var snapshot []sourcePhysicalSealedFile
	for _, item := range []struct {
		path string
		raw  []byte
		info os.FileInfo
	}{{p.Journal, hold.journalRaw, hold.journalInfo}, {p.Journal + ".recovery-completion", hold.headerRaw, hold.headerInfo},
		{hold.payload, hold.payloadRaw, hold.payloadInfo}, {pendingPath, pending, pendingInfo}} {
		stat := item.info.Sys().(*syscall.Stat_t)
		snapshot = append(snapshot, sourcePhysicalSealedFile{Record: sourceRecord{Path: item.path, Device: uint64(stat.Dev), Inode: stat.Ino, SHA256: sourcePhysicalSHA(item.raw), Mode: uint32(item.info.Mode().Perm()), UID: stat.Uid, GID: stat.Gid}, Size: item.info.Size(), Changed: stat.Ctim, Modified: stat.Mtim})
	}
	for _, suffix := range []string{".original-cut-journal", ".original-cut-result"} {
		path := p.Journal + suffix
		raw, info, err := readSourceCutObserverFile(path, 64<<10)
		if err != nil || suffix == ".original-cut-result" && sourcePhysicalSHA(raw) != p.OriginalCutSHA256 {
			return nil, errors.Join(errors.New("original repaired-file cut evidence differs"), err)
		}
		stat := info.Sys().(*syscall.Stat_t)
		snapshot = append(snapshot, sourcePhysicalSealedFile{Record: sourceRecord{Path: path, Device: uint64(stat.Dev), Inode: stat.Ino, SHA256: sourcePhysicalSHA(raw), Mode: 0600}, Size: info.Size(), Changed: stat.Ctim, Modified: stat.Mtim})
	}
	if _, err := os.Lstat(p.Journal + ".original-cut-pending"); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(errors.New("eligible repaired-file original cut must remain unguarded"), err)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	return &sourcePhysicalImage{journalInfo: hold.journalInfo, journalSHA: sourcePhysicalSHA(hold.journalRaw), payloadInfo: hold.headerInfo, payloadSHA: sourcePhysicalSHA(raw), pendingInfo: pendingInfo, pendingSHA: sourcePhysicalSHA(pending)}, nil
}

// Snapshot complete bytes separately from native Close. A syntactically complete
// envelope under pending remains unusable, including after this root is killed.
func (p *sourceRecoveryPhysicalProfile) image(unit string) (*sourcePhysicalImage, error) {
	if p.Window == "repaired-body-before-record" {
		return p.repairedImage()
	}
	original, journalInfo, err := readSourceCutObserverFile(p.Journal, 4<<20)
	if err != nil || sourcePhysicalSHA(original) != p.JournalSHA256 {
		return nil, errors.Join(errors.New("pinned immutable original recovery prefix differs"), err)
	}
	before, err := decodeSourceRecoveryPrefix(original, sourceRecoverableInitialRecords)
	if err != nil || len(before) != p.PrefixRecords || before[len(before)-1].Phase != p.LastPhase {
		return nil, errors.Join(errors.New("original recovery frontier differs"), err)
	}
	first, full := "initial", 55
	if p.initial() {
		first, full = sourceRecoverableInitialPhase, sourceRecoverableInitialRecords
	}
	if before[0].Phase != first {
		return nil, errors.New("original recovery format differs from selected producer")
	}
	completionPath := p.Journal + ".recovery-completion"
	completion, completionInfo, err := readSourceCutObserverFile(completionPath, 4<<20)
	if err != nil {
		return nil, err
	}
	operation, command := "replacement", "replace-source"
	if p.initial() {
		operation, command = "initial", "install-initial-stopped"
	}
	transcript, err := decodeSourceRecoveryCompletion(completion, original, journalInfo, p.Journal, operation, p.Generation, p.BindingSHA256)
	if err != nil {
		return nil, errors.Join(errors.New("actual complete separate recovery envelope required"), err)
	}
	records, err := decodeSourceRecoveryPrefix(transcript, sourceRecoverableInitialRecords)
	if err != nil || len(records) != full || records[len(records)-1].SHA256 != p.BindingSHA256 {
		return nil, errors.Join(errors.New("recovery complete binding differs"), err)
	}
	plan := &sourcePhysicalCut{window: "journal-complete-before-close", canonical: &sourceCanonicalPhysicalProfile{sourceCutObserverProfile: sourceCutObserverProfile{Arguments: []string{"installation", command}, ProgramSHA256: p.ProgramSHA256}, Generation: p.Generation}}
	inventory, _, err := plan.canonicalCompletedImage(records, p.root, unit)
	if err != nil {
		return nil, err
	}
	pendingPath, schema := p.pending()
	pending, pendingInfo, err := readSourceCutObserverFile(pendingPath, 4096)
	if err != nil || !bytes.Equal(pending, []byte(schema+"\n"+p.BindingSHA256+"\n")) {
		return nil, errors.Join(errors.New("original recovery pending marker differs"), err)
	}
	native := pendingInfo.Sys().(*syscall.Stat_t)
	if err := sourcePhysicalBirthTriple(records, sourceRecord{Path: pendingPath, Device: uint64(native.Dev), Inode: native.Ino, SHA256: sourcePhysicalSHA(pending), Mode: 0600}, "file"); err != nil {
		return nil, err
	}
	var snapshot []sourcePhysicalSealedFile
	for _, item := range []struct {
		path string
		raw  []byte
		info os.FileInfo
	}{
		{p.Journal, original, journalInfo}, {completionPath, completion, completionInfo}, {pendingPath, pending, pendingInfo},
	} {
		stat := item.info.Sys().(*syscall.Stat_t)
		snapshot = append(snapshot, sourcePhysicalSealedFile{Record: sourceRecord{Path: item.path, Device: uint64(stat.Dev), Inode: stat.Ino, SHA256: sourcePhysicalSHA(item.raw), Mode: 0600}, Size: item.info.Size(), Changed: stat.Ctim, Modified: stat.Mtim})
	}
	for _, suffix := range []string{".original-cut-journal", ".original-cut-result"} {
		path := p.Journal + suffix
		raw, info, err := readSourceCutObserverFile(path, 64<<10)
		if err != nil || suffix == ".original-cut-result" && sourcePhysicalSHA(raw) != p.OriginalCutSHA256 {
			return nil, errors.Join(errors.New("original eligible cut evidence differs"), err)
		}
		stat := info.Sys().(*syscall.Stat_t)
		snapshot = append(snapshot, sourcePhysicalSealedFile{Record: sourceRecord{Path: path, Device: uint64(stat.Dev), Inode: stat.Ino, SHA256: sourcePhysicalSHA(raw), Mode: 0600}, Size: info.Size(), Changed: stat.Ctim, Modified: stat.Mtim})
	}
	if _, err := os.Lstat(p.Journal + ".original-cut-pending"); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(errors.New("eligible original cut must remain unguarded"), err)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	return &sourcePhysicalImage{journalInfo: journalInfo, journalSHA: sourcePhysicalSHA(original), payloadInfo: completionInfo, payloadSHA: sourcePhysicalSHA(bytes.Join([][]byte{inventory, raw}, []byte{'\n'})), pendingInfo: pendingInfo, pendingSHA: sourcePhysicalSHA(pending)}, nil
}

func (p *sourcePhysicalCut) matchRecovery(tid int, registers *syscall.PtraceRegs, journal, root string) (bool, *sourcePhysicalImage, error) {
	profile := p.recovery
	if journal != profile.Journal || root != profile.root {
		return false, nil, errors.New("original root recovery paths differ")
	}
	if p.window == "repaired-body-before-record" {
		hold := profile.repairedHold()
		matches, err := hold.match(tid, registers)
		if err != nil || !matches {
			return false, nil, err
		}
	}
	if p.window == "pending-before-unlink" {
		path, err := sourcePhysicalString(tid, registers.Rax, registers.Rbx, 4096)
		pending, _ := profile.pending()
		if err != nil || path != pending {
			return false, nil, err
		}
	}
	image, err := profile.image("/etc/systemd/system/ardents-source.service")
	if err != nil {
		return false, nil, err
	}
	held := p.window != "pending-before-unlink"
	if held {
		if err := sourcePhysicalBirthsCloseReceiver(tid, registers.Rax, image.payloadInfo); err != nil {
			return false, nil, err
		}
	} else if err := verifySourcePhysicalRootFDs(tid, image); err != nil {
		return false, nil, err
	}
	for _, path := range []string{journal, journal + ".recovery-completion"} {
		if err := probeSourceRefusalLease(path, "", 0600, held); err != nil {
			return false, nil, err
		}
	}
	if err := probeSourceRefusalLease(filepath.Join(profile.Arguments[3], ".ardents-release-decision-lock"), "", 0600, true); err != nil {
		return false, nil, err
	}
	return true, image, nil
}

func (p *sourcePhysicalCut) verifyRecovery(before *sourcePhysicalImage) error {
	now, err := p.recovery.image("/etc/systemd/system/ardents-source.service")
	if err != nil || before == nil || !sameSourceFile(before.journalInfo, now.journalInfo) || !sameSourceFile(before.payloadInfo, now.payloadInfo) || !sameSourceFile(before.pendingInfo, now.pendingInfo) || before.journalSHA != now.journalSHA || before.payloadSHA != now.payloadSHA || before.pendingSHA != now.pendingSHA {
		return errors.Join(errors.New("original recovery image changed through root process cut"), err)
	}
	for _, path := range []string{p.recovery.Journal, p.recovery.Journal + ".recovery-completion", filepath.Join(p.recovery.Arguments[3], ".ardents-release-decision-lock")} {
		if err := probeSourceRefusalLease(path, "", 0600, false); err != nil {
			return err
		}
	}
	return nil
}

func TestSourceRecoveryPhysicalCutObserverNative(t *testing.T) {
	mode := os.Getenv("ARDENTS_SOURCE_RECOVERY_PHYSICAL_OBSERVER_MODE")
	if mode == "" {
		return
	}
	if mode != "inspect-recovery-physical" && mode != "installed-recovery-physical" || os.Getenv("ARDENTS_SOURCE_PHYSICAL_OBSERVER_WINDOW") != "" || os.Getenv("ARDENTS_SOURCE_CANONICAL_PHYSICAL_OBSERVER_MODE") != "" {
		t.Fatal("distinct root recovery process-cut purpose required")
	}
	program, root := os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM"), os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_ROOT")
	profile, err := readSourceRecoveryPhysicalProfile(os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE"), os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE_SHA256"), program, root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := observeSourceCut(program, root, os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT"), "unsafe-root-cut", nil, mode == "inspect-recovery-physical", 2, sourceRecoveryPhysicalWindow(profile))
	raw, encoded := json.Marshal(result)
	var reported error
	if encoded == nil {
		_, reported = fmt.Fprintln(os.Stdout, "SOURCE_RECOVERY_PHYSICAL_OBSERVATION "+string(raw))
	}
	if err := errors.Join(err, encoded, reported); err != nil {
		t.Fatal(err)
	}
}
