//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
)

// Outside observation only. The runner must retain the original PID1 Source
// before launching stop-source: it can already have exited at the held entry.
// Neither this image nor root reaps supply that separate original join.
type sourceRetirementPhysicalProfile struct {
	sourceCutObserverProfile
	Window             string `json:"window"`
	Attempt            uint32 `json:"attempt"`
	Generation         string `json:"generation"`
	BindingSHA256      string `json:"binding_sha256"`
	StartSHA256        string `json:"start_sha256"`
	StartJournalSHA256 string `json:"start_journal_sha256"`
	GID                uint32 `json:"gid"`
	root               string
}

func readSourceRetirementPhysicalProfile(path, pin, program, root string) (*sourceRetirementPhysicalProfile, error) {
	if !sourcePath(path) || !sourcePath(program) || filepath.Base(program) != ProgramName || !sourcePath(root) || !sourceCutObserverSHA(pin) {
		return nil, errors.New("exact retirement physical profile/program/root required")
	}
	raw, _, err := readSourceCutObserverFile(path, 64<<10)
	if err != nil || sourcePhysicalSHA(raw) != pin {
		return nil, errors.Join(errors.New("original retirement profile pin differs"), err)
	}
	p := new(sourceRetirementPhysicalProfile)
	if err := json.Unmarshal(raw, p); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return nil, errors.Join(errors.New("one canonical retirement physical profile required"), err)
	}
	p.sha256, p.root = pin, root
	if sourceRetirementPhysicalWindow(p) == nil {
		return nil, errors.New("finite exact retirement command/window required")
	}
	return p, nil
}

func sourceRetirementPhysicalWindow(p *sourceRetirementPhysicalProfile) *sourcePhysicalCut {
	if p == nil || p.Schema != "source-retirement-physical-cut-observer-v1" || !sourcePath(p.root) || len(p.Arguments) != 4 ||
		p.Arguments[0] != "installation" || p.Arguments[1] != "stop-source" || !sourcePath(p.Arguments[2]) || p.Arguments[3] != filepath.Join(p.root, "binding.json") ||
		p.Attempt >= sourceAttemptLimit || p.GID < 100 || p.GID > 999 || p.Journal != filepath.Join(sourceAttemptNumber(p.Attempt).directory(p.root), "source-retirement-journal") {
		return nil
	}
	for _, pin := range []string{p.sha256, p.ProgramSHA256, p.PublicReceiptSHA256, p.Generation, p.BindingSHA256, p.StartSHA256, p.StartJournalSHA256} {
		if !sourceCutObserverSHA(pin) {
			return nil
		}
	}
	const owner = "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure.(*sourceStopStorage)."
	count, last, symbol, next := sourceRetirementPhysicalCount(p.Window), "retired", owner+"close", ""
	if p.Window == "native-before-joined" {
		last, symbol, next = "domain-joined", owner+"record", "native-joined"
	} else if p.Window == "pending-before-unlink" {
		symbol = "os.Remove"
	}
	if count == 0 || p.PrefixRecords != count || p.LastPhase != last {
		return nil
	}
	return &sourcePhysicalCut{window: p.Window, symbol: symbol, lastPhase: last, nextPhase: next, records: count, root: true, killRoot: true, externalCut: true, retirement: p}
}

type sourceRetirementPhysicalSnapshot struct {
	image   *sourcePhysicalImage
	archive *sourceStartArchive
	result  *sourceRetirementResult
}

func (p *sourceRetirementPhysicalProfile) image() (*sourceRetirementPhysicalSnapshot, error) {
	number := sourceAttemptNumber(p.Attempt)
	directory := number.directory(p.root)
	guard, _, err := readSourceCutObserverFile(filepath.Join(directory, "start-guard.json"), 64<<10)
	if err != nil {
		return nil, err
	}
	var intent sourceStartIntent
	_, _, intentSchema, archiveSchema := number.schema()
	if !sourceCanonicalRecord(guard, &intent) || intent.Schema != intentSchema || intent.Attempt != p.Attempt || intent.UID != p.GID || intent.GID != p.GID ||
		intent.BindingSHA256 != p.BindingSHA256 || !sourceCutObserverSHA(intent.InitialHistorySHA256) || !sourceCutObserverSHA(intent.TargetsSHA256) ||
		p.Attempt == 0 && intent.PredecessorSHA256 != "" || p.Attempt != 0 && !sourceCutObserverSHA(intent.PredecessorSHA256) {
		return nil, errors.New("original retirement startup intent differs")
	}
	socketRaw, _, err := readSourceCutObserverFile(filepath.Join(directory, "start-socket.json"), 64<<10)
	if err != nil {
		return nil, err
	}
	var socket sourceSocketRecord
	socketSchema := "ardents-source-first-start-socket-v1"
	if number != 0 {
		socketSchema = "ardents-source-invocation-socket-v1"
	}
	if !sourceCanonicalRecord(socketRaw, &socket) || socket.Schema != socketSchema || socket.GuardSHA256 != digest(guard) || socket.GID != p.GID {
		return nil, errors.New("original retirement startup socket record differs")
	}
	archiveRaw, archiveInfo, err := readSourceCutObserverFile(filepath.Join(directory, "start-archived.json"), 64<<10)
	if err != nil || sourcePhysicalSHA(archiveRaw) != p.StartSHA256 {
		return nil, errors.Join(errors.New("exact original startup archive pin required"), err)
	}
	archive := new(sourceStartArchive)
	if !sourceCanonicalRecord(archiveRaw, archive) || archive.Schema != archiveSchema || archive.Generation != p.Generation || archive.BindingSHA256 != p.BindingSHA256 ||
		archive.GuardSHA256 != digest(guard) || archive.PID == 0 || archive.ProcessDevice == 0 || archive.ProcessInode == 0 || archive.CgroupDevice == 0 || archive.CgroupInode == 0 ||
		archive.Manager == "" || archive.StartedRealtime == 0 || archive.StartedMonotonic == 0 {
		return nil, errors.New("original retirement startup archive binding differs")
	}
	id, err := hex.DecodeString(archive.InvocationID)
	if err != nil || len(id) != 16 || hex.EncodeToString(id) != archive.InvocationID || bytes.Equal(id, make([]byte, 16)) {
		return nil, errors.Join(errors.New("original retirement invocation required"), err)
	}
	var invocation [16]byte
	copy(invocation[:], id)
	frame, err := completion.Encode(invocation, p.Generation, p.BindingSHA256)
	if err != nil {
		return nil, err
	}
	start, _, err := readSourceCutObserverFile(number.journal(p.root), 128<<10)
	if err != nil || sourcePhysicalSHA(start) != p.StartJournalSHA256 {
		return nil, errors.Join(errors.New("original complete startup journal pin differs"), err)
	}
	if _, err := decodeSourceStartRecords(start, number, number.journal(p.root), directory, p.BindingSHA256, digest(guard), digest(socketRaw), digest(archiveRaw), digest(frame[:]), p.GID); err != nil {
		return nil, err
	}
	var startup []sourceStartEntry
	for _, line := range bytes.Split(start[:len(start)-1], []byte{'\n'}) {
		var record sourceStartEntry
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, err
		}
		startup = append(startup, record)
	}
	if startup[9].Device != socket.Device || startup[9].Inode != socket.Inode {
		return nil, errors.New("original startup socket birth differs")
	}
	files, err := sourceStartupCompletedPhysicalFiles(startup)
	if err != nil {
		return nil, err
	}
	journal, journalInfo, err := readSourceCutObserverFile(p.Journal, 128<<10)
	if err != nil {
		return nil, err
	}
	lines := bytes.Split(bytes.TrimSuffix(journal, []byte{'\n'}), []byte{'\n'})
	if len(lines) != sourceRetirementPhysicalCount(p.Window) {
		return nil, errors.New("original retirement physical count differs")
	}
	var challenge, joined sourceStartEntry
	if json.Unmarshal(lines[4], &challenge) != nil || json.Unmarshal(lines[5], &joined) != nil {
		return nil, errors.New("original retirement frame records required")
	}
	resultSHA := ""
	var result *sourceRetirementResult
	payloadInfo := archiveInfo
	if len(lines) == 12 {
		raw, info, err := readSourceCutObserverFile(filepath.Join(directory, "source-retirement.json"), 64<<10)
		if err != nil {
			return nil, err
		}
		result = new(sourceRetirementResult)
		if !sourceCanonicalRecord(raw, result) || result.Schema != "ardents-source-retirement-v1" || result.Attempt != p.Attempt || result.Generation != p.Generation ||
			result.BindingSHA256 != p.BindingSHA256 || result.StartSHA256 != p.StartSHA256 || result.InvocationID != archive.InvocationID || result.PID != archive.PID ||
			result.Manager != archive.Manager || result.StartedRealtime != archive.StartedRealtime || result.StartedMonotonic != archive.StartedMonotonic ||
			result.ExitedRealtime < result.StartedRealtime || result.ExitedMonotonic < result.StartedMonotonic || result.Code != 1 || result.Status != 0 {
			return nil, errors.New("original retirement result/archive binding differs")
		}
		originalChallenge, err := sourceRetirementChallenge(result.Challenge)
		if err != nil {
			return nil, err
		}
		completed := sourceRetirementFrame(frame, originalChallenge)
		if result.CompletionSHA256 != digest(completed[:]) || challenge.SHA256 != digest(originalChallenge[:]) || joined.SHA256 != digest(completed[:]) {
			return nil, errors.New("original retirement result/frame commitment differs")
		}
		resultSHA, payloadInfo = sourcePhysicalSHA(raw), info
	}
	retirement, err := sourceRetirementPhysicalPrefix(journal, number, p.root, p.StartSHA256, challenge.SHA256, joined.SHA256, resultSHA, p.GID, p.Window)
	if err != nil {
		return nil, err
	}
	retiredFiles, err := sourceRetirementPhysicalFiles(retirement, p.GID)
	if err != nil {
		return nil, err
	}
	files = append(files, retiredFiles...)
	for _, target := range []struct {
		path, pin string
		mode      uint32
	}{
		{filepath.Join(p.root, "binding.json"), p.BindingSHA256, 0644},
		{filepath.Join(p.root, "generations", p.Generation, ProgramName), p.ProgramSHA256, 0555},
	} {
		info, err := os.Lstat(target.path)
		if err != nil {
			return nil, err
		}
		native := info.Sys().(*syscall.Stat_t)
		record := sourceRecord{Path: target.path, Device: uint64(native.Dev), Inode: native.Ino, SHA256: target.pin, Mode: target.mode}
		body, observed, err := readSourceRecoveryHeldPayload(target.path, record)
		if err != nil || sourcePhysicalSHA(body) != target.pin {
			return nil, errors.Join(errors.New("selected retirement resource bytes differ from original pin"), err)
		}
		native = observed.Sys().(*syscall.Stat_t)
		files = append(files, sourcePhysicalSealedFile{Record: record, Size: observed.Size(), Changed: native.Ctim, Modified: native.Mtim})
	}
	encoded, err := json.Marshal(files)
	if err != nil {
		return nil, err
	}
	pending, pendingInfo, err := readSourceCutObserverFile(filepath.Join(directory, "source-retirement-pending"), 4096)
	if err != nil {
		return nil, err
	}
	return &sourceRetirementPhysicalSnapshot{image: &sourcePhysicalImage{journalInfo: journalInfo, journalSHA: sourcePhysicalSHA(journal), payloadInfo: payloadInfo, payloadSHA: sourcePhysicalSHA(encoded), pendingInfo: pendingInfo, pendingSHA: sourcePhysicalSHA(pending)}, archive: archive, result: result}, nil
}

func (p *sourcePhysicalCut) matchRetirement(tid int, registers *syscall.PtraceRegs, journal, root string) (bool, *sourcePhysicalImage, error) {
	profile := p.retirement
	if journal != profile.Journal || root != profile.root {
		return false, nil, errors.New("exact original retirement paths required")
	}
	if p.window == "native-before-joined" {
		phase, err := sourcePhysicalString(tid, registers.Rbx, registers.Rcx, 128)
		if err != nil || phase != "native-joined" {
			return false, nil, err
		}
		path, err := sourcePhysicalString(tid, registers.Rdi, registers.Rsi, 4096)
		if err != nil || path != sourceAttemptNumber(profile.Attempt).directory(root) {
			return false, nil, err
		}
	} else if p.window == "pending-before-unlink" {
		path, err := sourcePhysicalString(tid, registers.Rax, registers.Rbx, 4096)
		if err != nil || path != filepath.Join(sourceAttemptNumber(profile.Attempt).directory(root), "source-retirement-pending") {
			return false, nil, err
		}
	}
	now, err := profile.image()
	if err != nil {
		return false, nil, err
	}
	held := p.window != "pending-before-unlink"
	if held {
		if err := sourcePhysicalLiveWriterReceiver(tid, registers.Rax, now.image.journalInfo, reflect.TypeFor[sourceStopStorage]()); err != nil {
			return false, nil, err
		}
	} else if err := verifySourcePhysicalRootFDs(tid, now.image); err != nil {
		return false, nil, err
	}
	if err := probeSourceRefusalLease(journal, "", 0600, held); err != nil {
		return false, nil, err
	}
	return true, now.image, nil
}

func (p *sourcePhysicalCut) verifyRetirement(before *sourcePhysicalImage) error {
	now, err := p.retirement.image()
	if err != nil || !sameSourcePhysicalImage(before, now.image) {
		return errors.Join(errors.New("original retirement image changed through root cut"), err)
	}
	return probeSourceRefusalLease(p.retirement.Journal, "", 0600, false)
}

func TestSourceRetirementPhysicalCutObserverNative(t *testing.T) {
	mode := os.Getenv("ARDENTS_SOURCE_RETIREMENT_PHYSICAL_OBSERVER_MODE")
	if mode == "" {
		return
	}
	if mode != "inspect-retirement-physical" && mode != "installed-retirement-physical" || os.Getenv("ARDENTS_SOURCE_STARTUP_PHYSICAL_OBSERVER_MODE") != "" || os.Getenv("ARDENTS_SOURCE_RECOVERY_PHYSICAL_OBSERVER_MODE") != "" || os.Getenv("ARDENTS_SOURCE_CANONICAL_PHYSICAL_OBSERVER_MODE") != "" || os.Getenv("ARDENTS_SOURCE_PHYSICAL_OBSERVER_WINDOW") != "" {
		t.Fatal("distinct retirement physical purpose required")
	}
	program, root := os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM"), os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_ROOT")
	p, err := readSourceRetirementPhysicalProfile(os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE"), os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE_SHA256"), program, root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := observeSourceCut(program, root, os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT"), "unsafe-root-cut", nil, mode == "inspect-retirement-physical", 2, sourceRetirementPhysicalWindow(p))
	raw, encoded := json.Marshal(result)
	var reported error
	if encoded == nil {
		_, reported = fmt.Fprintln(os.Stdout, "SOURCE_RETIREMENT_PHYSICAL_OBSERVATION "+string(raw))
	}
	if err := errors.Join(err, encoded, reported); err != nil {
		t.Fatal(err)
	}
}
