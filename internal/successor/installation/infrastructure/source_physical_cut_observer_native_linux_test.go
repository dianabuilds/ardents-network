//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	case "root-journal-before-sync", "root-file-born-before-record", "root-file-ready-before-record", "root-file-complete-before-close":
		plan := sourcePhysicalWindow(strings.TrimPrefix(window, "root-"))
		plan.window, plan.root = window, true
		return plan
	default:
		return nil
	}
}

func (p *sourcePhysicalCut) valid() bool {
	exact := sourcePhysicalWindow(p.window)
	return exact != nil && *p == *exact
}

type sourcePhysicalImage struct {
	journalInfo, payloadInfo os.FileInfo
	journalSHA, payloadSHA   string
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
	if p.absent {
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
	return true, image, nil
}

func (p *sourcePhysicalCut) verifyRetained(journal, root string, before *sourcePhysicalImage) error {
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
