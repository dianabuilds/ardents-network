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

// A separate physical observation purpose. It cannot produce the eligible ACK
// boundary proof, supply Source authority or accept its own bootstrap receipt.
type sourceCanonicalPhysicalProfile struct {
	sourceCutObserverProfile
	Window     string `json:"window"`
	Generation string `json:"generation"`
	root       string
}

func readSourceCanonicalPhysicalProfile(path, pin, program, root string) (*sourceCanonicalPhysicalProfile, error) {
	if !sourcePath(path) || !sourcePath(root) || !sourceCutObserverSHA(pin) {
		return nil, errors.New("original physical declaration path/pin required")
	}
	raw, _, err := readSourceCutObserverFile(path, 64<<10)
	if err != nil || sourcePhysicalSHA(raw) != pin {
		return nil, errors.Join(errors.New("original physical declaration pin differs"), err)
	}
	p := new(sourceCanonicalPhysicalProfile)
	if err := json.Unmarshal(raw, p); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return nil, errors.Join(errors.New("one exact canonical physical declaration required"), err)
	}
	if err := validateSourceCutObserverCommand(&p.sourceCutObserverProfile, program, root, "source-physical-cut-observer-v1"); err != nil {
		return nil, err
	}
	p.sha256, p.root = pin, root
	if sourceCanonicalPhysicalWindow(p) == nil {
		return nil, errors.New("finite canonical physical window required")
	}
	return p, nil
}

func sourceCanonicalPhysicalWindow(p *sourceCanonicalPhysicalProfile) *sourcePhysicalCut {
	if p == nil || !sourcePath(p.root) || !sourceCutObserverSHA(p.sha256) || !sourceCutObserverSHA(p.Generation) || len(p.Arguments) != 8 {
		return nil
	}
	program := filepath.Join(p.Arguments[2], ProgramName)
	if validateSourceCutObserverCommand(&p.sourceCutObserverProfile, program, p.root, "source-physical-cut-observer-v1") != nil {
		return nil
	}
	initial := p.Arguments[1] == "install-initial-stopped"
	bornCount := 13
	if initial {
		bornCount = 70
	} else if p.Journal != p.root+".source-replacement-"+p.Generation+"-journal" {
		return nil
	}
	plan := &sourcePhysicalCut{window: p.Window, symbol: sourceRecoveryHoldRecordSymbol, canonical: p, records: p.PrefixRecords}
	switch p.Window {
	case "account-before-complete-birth":
		plan.lastPhase, plan.nextPhase = "account-born", "account-born"
		if !initial || p.PrefixRecords != 24 {
			return nil
		}
	case "directory-born-before-record":
		plan.lastPhase, plan.nextPhase = "directory-intent", "directory-born"
		if !initial || p.PrefixRecords != 42 {
			return nil
		}
	case "file-born-before-record":
		plan.lastPhase, plan.nextPhase = "file-intent", "file-born"
		if p.PrefixRecords != bornCount-1 {
			return nil
		}
	case "file-ready-before-record":
		plan.lastPhase, plan.nextPhase = "file-born", "file-ready"
		if p.PrefixRecords != bornCount {
			return nil
		}
	case "file-born-before-journal-sync":
		plan.symbol = "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure.(*sourceBirths).sync"
		plan.lastPhase = "file-born"
		if p.PrefixRecords != bornCount {
			return nil
		}
	case "seal-before-sealed":
		count := 48
		if initial {
			count = 105
		}
		plan.lastPhase, plan.nextPhase = "seal-intent", "sealed"
		if p.PrefixRecords != count {
			return nil
		}
	case "binding-before-record", "unit-before-record":
		count := 50
		plan.lastPhase, plan.nextPhase = "selection-intent", "selection-written"
		if p.Window == "unit-before-record" {
			count, plan.lastPhase, plan.nextPhase = 52, "unit-intent", "unit-written"
		}
		if initial {
			count, plan.lastPhase, plan.nextPhase = 109, "file-ready", "selected"
			if p.Window == "unit-before-record" {
				count, plan.nextPhase = 113, "reload-intent"
			}
		}
		if p.PrefixRecords != count {
			return nil
		}
	case "reload-before-verified", "journal-complete-before-close", "pending-before-unlink":
		count := 54
		plan.lastPhase, plan.nextPhase = "reload-intent", "replacement-verified"
		if initial {
			count, plan.nextPhase = 114, "stopped-verified"
		}
		if p.Window != "reload-before-verified" {
			count++
			plan.symbol = "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure.(*sourceBirths).close"
			plan.lastPhase, plan.nextPhase = plan.nextPhase, ""
			if p.Window == "pending-before-unlink" {
				plan.symbol = "os.Remove"
			}
		}
		if p.PrefixRecords != count {
			return nil
		}
	default:
		return nil
	}
	if p.LastPhase != plan.lastPhase {
		return nil
	}
	return plan
}

func (p *sourcePhysicalCut) canonicalTarget(root string) string {
	switch p.window {
	case "account-before-complete-birth":
		return "/etc/group"
	case "directory-born-before-record":
		return root
	case "pending-before-unlink":
		path, _ := p.canonicalPending()
		return path
	case "binding-before-record", "reload-before-verified", "journal-complete-before-close":
		return filepath.Join(root, "binding.json")
	case "unit-before-record":
		return "/etc/systemd/system/ardents-source.service"
	case "seal-before-sealed":
		return filepath.Join(root, "generations", p.canonical.Generation)
	default:
		return filepath.Join(root, "generations", p.canonical.Generation, ProgramName)
	}
}

func (p *sourcePhysicalCut) canonicalPending() (string, string) {
	if p.canonical.Arguments[1] == "install-initial-stopped" {
		return sourceInitialPendingPath(p.canonical.root), "ardents-source-initial-pending-v1"
	}
	return filepath.Join(p.canonical.root, "source-replacement-pending"), "ardents-source-replacement-pending-v1"
}

func (p *sourcePhysicalCut) canonicalPayload(last sourceRecord, root string) ([]byte, os.FileInfo, error) {
	path := filepath.Join(root, "generations", p.canonical.Generation, ProgramName)
	if last.Path != path || last.SHA256 != p.canonical.ProgramSHA256 || last.Mode != 0555 || last.UID != 0 || last.GID != 0 {
		return nil, nil, errors.New("original program record differs from pinned physical target")
	}
	if p.nextPhase == "file-born" {
		raw, info, err := readSourcePhysicalPayload(path)
		if err != nil || len(raw) != 0 || last.Device != 0 || last.Inode != 0 {
			return nil, nil, errors.Join(errors.New("actual empty unrecorded original birth required"), err)
		}
		return raw, info, nil
	}
	if p.window == "file-born-before-journal-sync" {
		raw, info, err := readSourcePhysicalPayload(path)
		if err != nil || len(raw) != 0 {
			return nil, nil, errors.Join(errors.New("actual empty born program before journal Sync required"), err)
		}
		native := info.Sys().(*syscall.Stat_t)
		if last.Device != uint64(native.Dev) || last.Inode != native.Ino {
			return nil, nil, errors.New("actual pre-Sync born program inode differs")
		}
		return raw, info, nil
	}
	raw, info, err := readSourceRecoveryHeldPayload(path, last)
	if err != nil || sourcePhysicalSHA(raw) != p.canonical.ProgramSHA256 {
		return nil, nil, errors.Join(errors.New("actual same original program body before ready required"), err)
	}
	return raw, info, nil
}

func (p *sourcePhysicalCut) matchCanonical(tid int, registers *syscall.PtraceRegs, journal, root string) (bool, *sourcePhysicalImage, error) {
	if journal != p.canonical.Journal || root != p.canonical.root {
		return false, nil, errors.New("original physical root/journal differs")
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
	count := bytes.Count(raw, []byte{'\n'})
	preimage := p.needsPublicationPreimage() && count == p.records-1
	if count != p.records && !preimage {
		return false, nil, nil
	}
	records, err := decodeSourceRecoveryPrefix(raw, sourceRecoverableInitialRecords)
	if err != nil || len(records) != count || !preimage && records[len(records)-1].Phase != p.lastPhase {
		return false, nil, errors.Join(errors.New("actual canonical physical prefix differs"), err)
	}
	native := info.Sys().(*syscall.Stat_t)
	firstPhase := "initial"
	if p.canonical.Arguments[1] == "install-initial-stopped" {
		firstPhase = sourceRecoverableInitialPhase
	}
	if records[0] != (sourceRecord{Phase: firstPhase, Path: journal, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}) {
		return false, nil, errors.New("actual original journal inode/header required")
	}
	if preimage {
		phase, err := sourcePhysicalString(tid, registers.Rbx, registers.Rcx, 64)
		if err != nil {
			return false, nil, err
		}
		path, err := sourcePhysicalString(tid, registers.Rdi, registers.Rsi, 4096)
		if err != nil || phase != p.lastPhase || path != p.canonicalTarget(root) {
			return false, nil, errors.Join(errors.New("actual original publication intent entry required"), err)
		}
		if err := p.capturePublicationPreimage(records, path, sourcePhysicalSHA(raw)); err != nil {
			return false, nil, err
		}
		return false, nil, nil
	}
	if p.window == "pending-before-unlink" {
		path, err := sourcePhysicalString(tid, registers.Rax, registers.Rbx, 4096)
		if err != nil || path != p.canonicalTarget(root) {
			return false, nil, err
		}
	} else if p.window == "journal-complete-before-close" {
		if err := sourcePhysicalBirthsCloseReceiver(tid, registers.Rax, info); err != nil {
			return false, nil, err
		}
	} else if p.window == "file-born-before-journal-sync" {
		if err := sourcePhysicalJournalArgument(tid, registers.Rbx, info); err != nil {
			return false, nil, err
		}
	} else {
		phase, err := sourcePhysicalString(tid, registers.Rbx, registers.Rcx, 64)
		if err != nil {
			return false, nil, err
		}
		path, err := sourcePhysicalString(tid, registers.Rdi, registers.Rsi, 4096)
		target := p.canonicalTarget(root)
		if err != nil || phase != p.nextPhase || path != target {
			return false, nil, errors.Join(errors.New("actual first-program physical entry arguments differ"), err)
		}
	}
	body, payloadInfo, err := p.canonicalImage(records, root)
	if err != nil {
		return false, nil, err
	}
	pendingPath, pendingSchema := p.canonicalPending()
	pending, pendingInfo, err := readSourceCutObserverFile(pendingPath, 4096)
	if err != nil {
		return false, nil, errors.Join(errors.New("original pending marker/binding required"), err)
	}
	fields := bytes.Split(pending, []byte{'\n'})
	if len(fields) != 3 || string(fields[0]) != pendingSchema || !sourceCutObserverSHA(string(fields[1])) || len(fields[2]) != 0 {
		return false, nil, errors.New("actual original closed pending marker required")
	}
	// The binding is an output of this actual original operation, not a new
	// profile permission. Bind its current bytes/access/inode to its original
	// intent/born/ready triple before retaining it through physical teardown.
	pendingNative := pendingInfo.Sys().(*syscall.Stat_t)
	expectedPending := sourceRecord{Path: pendingPath, Device: uint64(pendingNative.Dev), Inode: pendingNative.Ino, SHA256: sourcePhysicalSHA(pending), Mode: 0600}
	if err := sourcePhysicalBirthTriple(records, expectedPending, "file"); err != nil {
		return false, nil, errors.Join(errors.New("original pending birth lineage differs"), err)
	}
	held := p.window != "pending-before-unlink"
	if err := probeSourceRefusalLease(journal, sourcePhysicalSHA(raw), 0600, held); err != nil {
		return false, nil, err
	}
	image := &sourcePhysicalImage{journalInfo: info, journalSHA: sourcePhysicalSHA(raw), payloadInfo: payloadInfo,
		payloadSHA: sourcePhysicalSHA(body), pendingInfo: pendingInfo, pendingSHA: sourcePhysicalSHA(pending)}
	if !held {
		if err := verifySourcePhysicalRootFDs(tid, image); err != nil {
			return false, nil, err
		}
	}
	return true, image, nil
}

func (p *sourcePhysicalCut) verifyCanonicalRetained(journal, root string, before *sourcePhysicalImage) error {
	raw, info, err := readSourceCutObserverFile(journal, 4<<20)
	if err != nil || before == nil || !sameSourceFile(before.journalInfo, info) || sourcePhysicalSHA(raw) != before.journalSHA {
		return errors.Join(errors.New("original canonical physical prefix changed after join"), err)
	}
	records, err := decodeSourceRecoveryPrefix(raw, sourceRecoverableInitialRecords)
	if err != nil || len(records) != p.records {
		return errors.Join(errors.New("original canonical physical count changed"), err)
	}
	body, payloadInfo, err := p.canonicalImage(records, root)
	if err != nil || !sameSourceFile(before.payloadInfo, payloadInfo) || sourcePhysicalSHA(body) != before.payloadSHA {
		return errors.Join(errors.New("unrecorded original program effect changed after join"), err)
	}
	pendingPath, _ := p.canonicalPending()
	pending, pendingInfo, err := readSourceCutObserverFile(pendingPath, 4096)
	if err != nil || !sameSourceFile(before.pendingInfo, pendingInfo) || sourcePhysicalSHA(pending) != before.pendingSHA {
		return errors.Join(errors.New("original pending marker changed after physical join"), err)
	}
	for _, suffix := range []string{".original-cut-journal", ".original-cut-result", ".original-cut-pending", ".recovery-completion"} {
		if _, err := os.Lstat(journal + suffix); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(errors.New("physical cut manufactured eligible completion evidence"), err)
		}
	}
	return probeSourceRefusalLease(journal, before.journalSHA, 0600, false)
}

func TestSourceCanonicalPhysicalCutObserverNative(t *testing.T) {
	mode := os.Getenv("ARDENTS_SOURCE_CANONICAL_PHYSICAL_OBSERVER_MODE")
	if mode == "" {
		return
	}
	if mode != "inspect-physical" && mode != "installed-physical" || os.Getenv("ARDENTS_SOURCE_PHYSICAL_OBSERVER_WINDOW") != "" || os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_NATIVE_RECORDS") != "" {
		t.Fatal("separate canonical physical observation purpose required")
	}
	program, root := os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM"), os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_ROOT")
	profile, err := readSourceCanonicalPhysicalProfile(os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE"), os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE_SHA256"), program, root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := observeSourceCut(program, root, os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT"), "unsafe-external-cut", nil, mode == "inspect-physical", 2, sourceCanonicalPhysicalWindow(profile))
	raw, encoded := json.Marshal(result)
	var reported error
	if encoded == nil {
		_, reported = fmt.Fprintln(os.Stdout, "SOURCE_CANONICAL_PHYSICAL_OBSERVATION "+string(raw))
	}
	if err := errors.Join(err, encoded, reported); err != nil {
		t.Fatal(err)
	}
}
