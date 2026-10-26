//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// This separate file-only profile owns a private container mount namespace:
// shell PID1, its own marker, and empty ext4 /var/lib and fixed-unit directory.
// It does not run inside an installed Source stand or supply accepting Release,
// State, retirement or cut proof. The actual lower consumer must reject a wrong
// pair before its deliberately missing pending file; matching pairs must reach
// that later read. No runtime path override or successful neighbor is supplied.
func TestSourceRecoveryPredecessorPairNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-pair-native-stand")
	pid1, pidErr := os.ReadFile("/proc/1/comm")
	if err != nil || pidErr != nil || string(marker) != "ardents-source-pair-native-v1\n" || strings.TrimSpace(string(pid1)) != "sh" || os.Geteuid() != 0 {
		t.Fatal("invalid environment: private shell-parent pair-control mount namespace required", err, pidErr)
	}
	for _, path := range []string{"/var/lib", "/etc/systemd/system"} {
		var fs syscall.Statfs_t
		if err := syscall.Statfs(path, &fs); err != nil || fs.Type != 0xef53 {
			t.Fatal("invalid environment: private ext4 pair-control mounts required", path, err)
		}
	}
	previousGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previousGC)
	for _, count := range []int{49, 51, 53} {
		for _, selectedNew := range []bool{false, true} {
			for _, unitNew := range []bool{false, true} {
				for _, failClose := range []bool{false, true} {
					name := fmt.Sprintf("prefix-%d-binding-new-%t-unit-new-%t-close-failure-%t", count, selectedNew, unitNew, failClose)
					if !t.Run(name, func(t *testing.T) {
						sourceRecoveryPairNativeCase(t, count, selectedNew, unitNew, failClose, name)
					}) {
						return // Preserve the failed fixture; do not cascade into it.
					}
				}
			}
		}
	}
}

func sourceRecoveryPairNativeCase(t *testing.T, count int, selectedNew, unitNew, failClose bool, name string) {
	t.Helper()
	const root = "/var/lib/install"
	const unitPath = "/etc/systemd/system/ardents-source.service"
	journal := sourceReplacementJournal(root, strings.Repeat("a", 64))
	for _, path := range []string{root, unitPath, journal} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("invalid environment: another fixture or installed resource exists", path, err)
		}
	}
	births, fixturePath := nativeSourceBirths(t, t.Context())
	if err := births.close(); err != nil {
		t.Fatal(err)
	}
	base := filepath.Dir(fixturePath)
	oldRaw := sourceBindingVector()
	oldBinding, err := decodeSourceBinding(oldRaw)
	if err != nil {
		t.Fatal(err)
	}
	oldBinding.Generation = strings.Repeat("f", 64)
	oldUnit, err := RenderSourceUnit([]byte(SourceTemplate), oldBinding.Request, oldBinding.Generation)
	if err != nil {
		t.Fatal("render original fixture unit", err)
	}
	oldBinding.UnitSHA256 = digest(oldUnit)
	oldRaw = sourceRecoveryPairNativeJSON(t, oldBinding)
	vector, binding, _ := sourceReplacementRecordVector(t)
	binding.PredecessorBindingSHA256 = digest(oldRaw)
	newUnit, err := RenderSourceUnit([]byte(SourceTemplate), binding.Configuration.Request, binding.Configuration.Generation)
	if err != nil {
		t.Fatal("render candidate fixture unit", err)
	}
	if binding.Configuration.UnitSHA256 != digest(newUnit) {
		t.Fatal("independent binding vector differs from rendered candidate unit")
	}
	candidate := &sourceReplacementCandidate{binding: binding, raw: sourceRecoveryPairNativeJSON(t, binding), unit: newUnit, files: map[string][]byte{}}
	for _, leaf := range sourceGenerationNames() {
		candidate.files[leaf] = []byte(leaf)
	}
	candidate.files["binding.json"], candidate.files["rendered-source.service"] = candidate.raw, newUnit
	sealed := filepath.Join(root, "generations", oldBinding.Generation)
	if err := os.MkdirAll(sealed, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(path string, raw []byte, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	archive := filepath.Join(sealed, "binding.json")
	rendered := filepath.Join(sealed, "rendered-source.service")
	selected := filepath.Join(root, "binding.json")
	write(archive, oldRaw, 0444)
	write(rendered, oldUnit, 0444)
	write(selected, oldRaw, 0644)
	write(unitPath, oldUnit, 0644)
	selectedInfo, err := os.Stat(selected)
	if err != nil {
		t.Fatal(err)
	}
	unitInfo, err := os.Stat(unitPath)
	if err != nil {
		t.Fatal(err)
	}
	wantedSelected, wantedUnit := oldRaw, oldUnit
	if selectedNew {
		wantedSelected = candidate.raw
		write(selected, wantedSelected, 0644)
	}
	if unitNew {
		wantedUnit = newUnit
		write(unitPath, wantedUnit, 0644)
	}
	write(journal, nil, 0600)
	journalInfo, err := os.Stat(journal)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := os.Open(journal)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	historyRoot := filepath.Join(base, "release-history")
	watched := map[string]bool{selected: true, unitPath: true, archive: true, rendered: true, journal: true, historyRoot: true}
	for _, leaf := range []string{selected, unitPath, archive, journal, historyRoot} {
		for parent := filepath.Dir(leaf); ; parent = filepath.Dir(parent) {
			watched[parent] = true
			if parent == "/" {
				break
			}
		}
	}
	before := sourceHistoryDescriptors(t, watched)
	v, err := release.Open(historyRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	history, err := openSourceHistory(t.Context(), v, historyRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer history.close()
	admitted, err := history.selectedCommitment(v, candidate.raw)
	if err != nil {
		t.Fatal(err)
	}
	records, err := decodeSourceRecoveryPrefix(vector, 55)
	if err != nil {
		t.Fatal(err)
	}
	bodies := sourceReplacementRecoveryBodies(candidate)
	hashes := map[string]string{}
	for path, body := range bodies {
		hashes[path] = digest(body)
	}
	for i := range records {
		r := &records[i]
		if r.Inode != 0 {
			r.Device = uint64(journalInfo.Sys().(*syscall.Stat_t).Dev)
		}
		switch r.Phase {
		case "initial":
			r.Inode = journalInfo.Sys().(*syscall.Stat_t).Ino
		case "replacement-admitted":
			r.SHA256 = admitted
		case "predecessor-binding":
			r.SHA256 = digest(oldRaw)
		case "predecessor-unit":
			r.SHA256 = digest(oldUnit)
		case "file-intent", "file-born", "file-ready", "selection-intent", "selection-written", "unit-intent", "unit-written":
			r.SHA256 = hashes[r.Path]
		case "reload-intent", "replacement-verified":
			r.SHA256 = digest(newUnit)
		}
		if r.Inode != 0 && r.Path == selected {
			r.Device = uint64(selectedInfo.Sys().(*syscall.Stat_t).Dev)
			r.Inode = selectedInfo.Sys().(*syscall.Stat_t).Ino
		}
		if r.Inode != 0 && r.Path == unitPath {
			r.Device = uint64(unitInfo.Sys().(*syscall.Stat_t).Dev)
			r.Inode = unitInfo.Sys().(*syscall.Stat_t).Ino
		}
	}
	var raw []byte
	for _, record := range records[:count] {
		raw = append(raw, sourceRecoveryPairNativeJSON(t, record)...)
	}
	if err := validateSourceReplacementPrefix(raw, binding, hashes, admitted, digest(oldUnit)); err != nil {
		t.Fatal("independent pair fixture did not reach valid prefix grammar", err)
	}
	write(journal, raw, 0600)
	prefix, err := openSourceRecoveryPrefix(t.Context(), journal, "replacement")
	if err != nil {
		t.Fatal(err)
	}
	defer prefix.close()
	// Deliberately untrusted boundary data isolates the lower consumer. It is
	// not a decoded original-cut result and grants no RecoverSource admission.
	prefix.cut = &sourceCutResult{Boundary: sourceTransitionMessage{Generation: binding.Configuration.Generation, BindingSHA256: digest(candidate.raw)}}
	old := &sourceSelection{ctx: t.Context(), binding: oldBinding, bindingPath: archive, sealed: sealed, files: map[string]*sourceFile{}, dirs: map[string]*os.File{}, identities: map[string]os.FileInfo{}}
	defer old.close()
	if _, err := old.read(archive, 32<<10, 0444, 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := old.read(rendered, 32<<10, 0444, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("actual original prefix exclusion absent", err)
	}
	if failClose {
		// The real consumer's later Close must retain this independent native
		// failure, even when its ordinary expected pair refusal is also present.
		if err := old.files[archive].file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	result, observed := openSourceRecoveryPredecessor(v, old, history, prefix, candidate)
	if result != nil || observed == nil || !old.closed || !history.closed {
		_ = result.close()
		t.Fatal("lower consumer accepted incomplete history or did not join its native owners", observed)
	}
	matching := selectedNew == (count > 50) && unitNew == (count > 52)
	var missing *os.PathError
	reachedPending := errors.As(observed, &missing) && errors.Is(missing, os.ErrNotExist) && missing.Path == filepath.Join(root, "source-replacement-pending")
	if matching {
		if !reachedPending || old.files[selected] == nil || old.files[unitPath] == nil {
			t.Fatal("matching pair did not reach the subsequent missing pending read", observed)
		}
	} else if !errors.Is(observed, ErrBinding) || reachedPending {
		t.Fatal("incorrect pair escaped actual consumer comparison", observed)
	}
	if old.files[selected] == nil || selectedNew == (count > 50) && old.files[unitPath] == nil {
		t.Fatal("refusal did not reach its actual native pair member")
	}
	expected := error(ErrBinding)
	if matching {
		expected = syscall.ENOENT
	}
	leaves := sourceRecoveryPairNativeErrorLeaves(observed)
	refusalOnly := len(leaves) == 1 && leaves[0] == expected
	if refusalOnly == failClose || failClose && (len(leaves) != 2 || leaves[0] != expected || leaves[1] != os.ErrClosed) {
		t.Fatal("expected refusal masked an independent native cleanup error", observed)
	}
	closed := errors.Join(old.close(), history.close(), prefix.close(), v.Close())
	cleanup := sourceRecoveryPairNativeErrorLeaves(closed)
	if failClose {
		if len(cleanup) != 1 || cleanup[0] != os.ErrClosed {
			t.Fatal("real retained Close failure changed or acquired another cleanup cause", closed)
		}
	} else if closed != nil {
		t.Fatal("independent native cleanup failed", closed)
	}
	if got := sourceHistoryDescriptors(t, watched); !reflect.DeepEqual(got, before) {
		t.Fatal("pair consumer retained descriptors before GC", before, got)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal("pair fixture retained original prefix exclusion after Close", err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	for path, wanted := range map[string][]byte{selected: wantedSelected, unitPath: wantedUnit, journal: raw} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, wanted) {
			t.Fatal("pair refusal or Close changed retained native evidence", path, err)
		}
	}
	// Retain every case; never restore a damaged input into another fixture.
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(journal, journal+"."+name); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(unitPath, unitPath+"."+name); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, root+"."+name); err != nil {
		t.Fatal(err)
	}
	t.Log("actual prefix-dependent consumer; matching pair reached later pending read, wrong pairs refused; native evidence and before-GC FD/flock release retained", count, matching)
}

func sourceRecoveryPairNativeJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

// Inspect the entire concrete error tree, including every cleanup branch.
// errors.Is/As alone can find the expected refusal while hiding a later failure.
func sourceRecoveryPairNativeErrorLeaves(err error) []error {
	if err == nil {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var leaves []error
		for _, child := range joined.Unwrap() {
			leaves = append(leaves, sourceRecoveryPairNativeErrorLeaves(child)...)
		}
		return leaves
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return sourceRecoveryPairNativeErrorLeaves(wrapped.Unwrap())
	}
	return []error{err}
}
