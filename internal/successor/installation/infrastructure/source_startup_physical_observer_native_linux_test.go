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
	"strconv"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
)

// Closed outside observation, never startup/recovery authority. The controller
// must retain the separate original PID1 Source pidfd/cgroup before requesting
// the cut; the tracer can join only its own original root and descendants.
type sourceStartupPhysicalProfile struct {
	sourceCutObserverProfile
	Window        string `json:"window"`
	Attempt       uint32 `json:"attempt"`
	Generation    string `json:"generation"`
	BindingSHA256 string `json:"binding_sha256"`
	GID           uint32 `json:"gid"`
	root          string
}

func readSourceStartupPhysicalProfile(path, pin, program, root string) (*sourceStartupPhysicalProfile, error) {
	if !sourcePath(path) || !sourcePath(root) || !sourceCutObserverSHA(pin) {
		return nil, errors.New("exact startup physical profile path/pin required")
	}
	raw, _, err := readSourceCutObserverFile(path, 64<<10)
	if err != nil || sourcePhysicalSHA(raw) != pin {
		return nil, errors.Join(errors.New("startup physical profile pin differs"), err)
	}
	p := new(sourceStartupPhysicalProfile)
	if err := json.Unmarshal(raw, p); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return nil, errors.Join(errors.New("one canonical startup physical profile required"), err)
	}
	p.sha256, p.root = pin, root
	if program != filepath.Join(p.argument(2), ProgramName) || sourceStartupPhysicalWindow(p) == nil {
		return nil, errors.New("finite canonical startup physical command required")
	}
	return p, nil
}

func (p *sourceStartupPhysicalProfile) argument(index int) string {
	if p == nil || index < 0 || index >= len(p.Arguments) {
		return ""
	}
	return p.Arguments[index]
}

func sourceStartupPhysicalWindow(p *sourceStartupPhysicalProfile) *sourcePhysicalCut {
	if p == nil || p.Schema != "source-startup-physical-cut-observer-v1" || !sourcePath(p.root) || len(p.Arguments) != 7 ||
		p.Arguments[0] != "installation" || p.Arguments[4] != "alpha" || !sourceCutObserverNetwork(p.Arguments[5]) ||
		p.Attempt >= uint32(sourceAttemptLimit) || p.GID < 100 || p.GID > 999 ||
		!sourceCutObserverSHA(p.sha256) || !sourceCutObserverSHA(p.ProgramSHA256) || !sourceCutObserverSHA(p.PublicReceiptSHA256) ||
		!sourceCutObserverSHA(p.Generation) || !sourceCutObserverSHA(p.BindingSHA256) {
		return nil
	}
	command := "start-source"
	if p.Attempt != 0 {
		command = "restart-source"
	}
	if p.Arguments[1] != command || p.Arguments[6] != filepath.Join(p.root, "binding.json") || p.Journal != sourceAttemptNumber(p.Attempt).journal(p.root) {
		return nil
	}
	for _, index := range []int{2, 3, 6} {
		if !sourcePath(p.Arguments[index]) {
			return nil
		}
	}
	count, last, next := 19, "file-ready", "archived"
	if p.Window == "ack-before-written" {
		count, last, next = 21, "ack-attempt", "ack-written"
	} else if p.Window != "archive-before-archived" {
		return nil
	}
	if p.PrefixRecords != count || p.LastPhase != last {
		return nil
	}
	return &sourcePhysicalCut{window: p.Window, symbol: "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure.(*sourceStartStorage).record",
		root: true, killRoot: true, externalCut: true, startup: p, records: count, lastPhase: last, nextPhase: next}
}

func (p *sourceStartupPhysicalProfile) image(live bool) (*sourcePhysicalImage, *sourceStartArchive, error) {
	directory := sourceAttemptNumber(p.Attempt).directory(p.root)
	journal, journalInfo, err := readSourceCutObserverFile(p.Journal, 128<<10)
	if err != nil {
		return nil, nil, err
	}
	guard, _, err := readSourceCutObserverFile(filepath.Join(directory, "start-guard.json"), 64<<10)
	if err != nil {
		return nil, nil, err
	}
	var intent sourceStartIntent
	_, _, intentSchema, archiveSchema := sourceAttemptNumber(p.Attempt).schema()
	if !sourceCanonicalRecord(guard, &intent) || intent.Schema != intentSchema || intent.Attempt != p.Attempt ||
		intent.BindingSHA256 != p.BindingSHA256 || intent.UID != p.GID || intent.GID != p.GID ||
		!sourceCutObserverSHA(intent.InitialHistorySHA256) || !sourceCutObserverSHA(intent.TargetsSHA256) ||
		p.Attempt == 0 && intent.PredecessorSHA256 != "" || p.Attempt != 0 && !sourceCutObserverSHA(intent.PredecessorSHA256) {
		return nil, nil, errors.New("original pending guard binding differs")
	}
	socketRaw, _, err := readSourceCutObserverFile(filepath.Join(directory, "start-socket.json"), 64<<10)
	if err != nil {
		return nil, nil, err
	}
	var socket sourceSocketRecord
	socketSchema := "ardents-source-first-start-socket-v1"
	if p.Attempt != 0 {
		socketSchema = "ardents-source-invocation-socket-v1"
	}
	if !sourceCanonicalRecord(socketRaw, &socket) || socket.Schema != socketSchema || socket.GuardSHA256 != digest(guard) || socket.GID != p.GID || socket.Device == 0 || socket.Inode == 0 {
		return nil, nil, errors.New("original startup socket record differs")
	}
	archiveRaw, archiveInfo, err := readSourceCutObserverFile(filepath.Join(directory, "start-archived.json"), 64<<10)
	if err != nil {
		return nil, nil, err
	}
	archive := new(sourceStartArchive)
	if !sourceCanonicalRecord(archiveRaw, archive) || archive.Schema != archiveSchema || archive.Generation != p.Generation ||
		archive.BindingSHA256 != p.BindingSHA256 || archive.GuardSHA256 != digest(guard) || archive.PID == 0 ||
		archive.ProcessDevice == 0 || archive.ProcessInode == 0 || archive.CgroupDevice == 0 || archive.CgroupInode == 0 ||
		archive.Manager == "" || archive.StartedRealtime == 0 || archive.StartedMonotonic == 0 {
		return nil, nil, errors.New("original startup archive binding differs")
	}
	id, err := hex.DecodeString(archive.InvocationID)
	if err != nil || len(id) != 16 || hex.EncodeToString(id) != archive.InvocationID || bytes.Equal(id, make([]byte, 16)) {
		return nil, nil, errors.Join(errors.New("original startup invocation differs"), err)
	}
	var invocation [16]byte
	copy(invocation[:], id)
	frame, err := completion.Encode(invocation, p.Generation, p.BindingSHA256)
	if err != nil {
		return nil, nil, err
	}
	records, err := sourceStartupPhysicalPrefix(journal, sourceAttemptNumber(p.Attempt), p.root, p.BindingSHA256, digest(guard), digest(socketRaw), digest(archiveRaw), digest(frame[:]), p.GID, p.Window)
	if err != nil || records[9].Device != socket.Device || records[9].Inode != socket.Inode {
		return nil, nil, errors.Join(errors.New("exact original startup physical prefix/socket required"), err)
	}
	files, err := sourceStartupPhysicalFiles(records)
	if err != nil {
		return nil, nil, err
	}
	var programInfo os.FileInfo
	for _, target := range []struct {
		path, pin string
		mode      uint32
	}{{filepath.Join(p.root, "binding.json"), p.BindingSHA256, 0644}, {filepath.Join(p.root, "generations", p.Generation, ProgramName), p.ProgramSHA256, 0555}} {
		info, err := os.Lstat(target.path)
		if err != nil {
			return nil, nil, err
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return nil, nil, errors.New("native selected startup resource required")
		}
		record := sourceRecord{Path: target.path, Device: uint64(native.Dev), Inode: native.Ino, SHA256: target.pin, Mode: target.mode}
		raw, info, err := readSourceRecoveryHeldPayload(target.path, record)
		if err != nil || sourcePhysicalSHA(raw) != target.pin {
			return nil, nil, errors.Join(errors.New("selected startup resource bytes/access differ"), err)
		}
		native = info.Sys().(*syscall.Stat_t)
		files = append(files, sourcePhysicalSealedFile{Record: record, Size: info.Size(), Changed: native.Ctim, Modified: native.Mtim})
		if target.mode == 0555 {
			programInfo = info
		}
	}
	if live {
		process, err := os.Lstat("/proc/" + strconv.FormatUint(uint64(archive.PID), 10))
		if err != nil || !process.IsDir() {
			return nil, nil, errors.Join(errors.New("original archived Source process is not alive"), err)
		}
		native := process.Sys().(*syscall.Stat_t)
		actual, err := os.Stat("/proc/" + strconv.FormatUint(uint64(archive.PID), 10) + "/exe")
		if err != nil || uint64(native.Dev) != archive.ProcessDevice || native.Ino != archive.ProcessInode || !os.SameFile(actual, programInfo) {
			return nil, nil, errors.Join(errors.New("original archived Source process/program differs"), err)
		}
		scope, err := os.Lstat("/sys/fs/cgroup/system.slice/ardents-source.service")
		if err != nil || !scope.IsDir() {
			return nil, nil, errors.Join(errors.New("original archived Source scope missing"), err)
		}
		native = scope.Sys().(*syscall.Stat_t)
		if uint64(native.Dev) != archive.CgroupDevice || native.Ino != archive.CgroupInode {
			return nil, nil, errors.New("original archived Source scope differs")
		}
	}
	raw, err := json.Marshal(files)
	if err != nil {
		return nil, nil, err
	}
	pending, pendingInfo, err := readSourceCutObserverFile(filepath.Join(directory, "source-start-pending"), 4096)
	if err != nil {
		return nil, nil, err
	}
	return &sourcePhysicalImage{journalInfo: journalInfo, journalSHA: sourcePhysicalSHA(journal), payloadInfo: archiveInfo, payloadSHA: sourcePhysicalSHA(raw), pendingInfo: pendingInfo, pendingSHA: sourcePhysicalSHA(pending)}, archive, nil
}

func (p *sourcePhysicalCut) matchStartup(tid int, registers *syscall.PtraceRegs, journal, root string) (bool, *sourcePhysicalImage, error) {
	profile := p.startup
	if journal != profile.Journal || root != profile.root {
		return false, nil, errors.New("exact startup physical paths required")
	}
	phase, err := sourcePhysicalString(tid, registers.Rbx, registers.Rcx, 128)
	if err != nil || phase != p.nextPhase {
		return false, nil, err
	}
	path, err := sourcePhysicalString(tid, registers.Rdi, registers.Rsi, 4096)
	wanted := sourceAttemptNumber(profile.Attempt).directory(root)
	if p.nextPhase == "archived" {
		wanted = filepath.Join(wanted, "start-archived.json")
	}
	if err != nil || path != wanted {
		return false, nil, err
	}
	image, _, err := profile.image(true)
	if err != nil {
		return false, nil, err
	}
	if err := sourcePhysicalLiveWriterReceiver(tid, registers.Rax, image.journalInfo, reflect.TypeFor[sourceStartStorage]()); err != nil {
		return false, nil, err
	}
	if err := probeSourceRefusalLease(journal, "", 0600, true); err != nil {
		return false, nil, err
	}
	return true, image, nil
}

func (p *sourcePhysicalCut) verifyStartup(before *sourcePhysicalImage) error {
	now, _, err := p.startup.image(false)
	if err != nil || !sameSourcePhysicalImage(before, now) {
		return errors.Join(errors.New("original startup image changed through root cut"), err)
	}
	return probeSourceRefusalLease(p.startup.Journal, "", 0600, false)
}

func TestSourceStartupPhysicalCutObserverNative(t *testing.T) {
	mode := os.Getenv("ARDENTS_SOURCE_STARTUP_PHYSICAL_OBSERVER_MODE")
	if mode == "" {
		return
	}
	if mode != "inspect-startup-physical" && mode != "installed-startup-physical" || os.Getenv("ARDENTS_SOURCE_RECOVERY_PHYSICAL_OBSERVER_MODE") != "" || os.Getenv("ARDENTS_SOURCE_CANONICAL_PHYSICAL_OBSERVER_MODE") != "" || os.Getenv("ARDENTS_SOURCE_PHYSICAL_OBSERVER_WINDOW") != "" {
		t.Fatal("distinct canonical startup physical purpose required")
	}
	program, root := os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM"), os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_ROOT")
	p, err := readSourceStartupPhysicalProfile(os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE"), os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE_SHA256"), program, root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := observeSourceCut(program, root, os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT"), "unsafe-root-cut", nil, mode == "inspect-startup-physical", 2, sourceStartupPhysicalWindow(p))
	raw, encoded := json.Marshal(result)
	var reported error
	if encoded == nil {
		_, reported = fmt.Fprintln(os.Stdout, "SOURCE_STARTUP_PHYSICAL_OBSERVATION "+string(raw))
	}
	if err := errors.Join(err, encoded, reported); err != nil {
		t.Fatal(err)
	}
}
