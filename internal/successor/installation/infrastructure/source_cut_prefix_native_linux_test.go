//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// Each count has its own genuinely joined controller and freshly published
// native proof. Only this fixture's control file is shortened, on its original
// inode. Complete result bytes remain available; no damaged fixture is repaired
// into another case or an accepting Source operation.
func sourceCutNativeControlPrefix(t *testing.T, ctx context.Context, boundary sourceTransitionMessage, program, scenario string) {
	t.Helper()
	count, err := strconv.Atoi(strings.TrimPrefix(scenario, "cut-proof-prefix-"))
	if err != nil || count < 1 || count > 13 {
		t.Fatal("finite original-cut prefix case required", scenario, err)
	}
	control, result, pending := sourceCutPaths(boundary.Journal)
	original, err := os.ReadFile(boundary.Journal)
	if err != nil {
		t.Fatal(err)
	}
	complete, err := os.ReadFile(control)
	if err != nil || len(complete) == 0 || complete[len(complete)-1] != '\n' {
		t.Fatal("complete original control required", err)
	}
	lines := bytes.SplitAfter(complete, []byte{'\n'})
	if len(lines) != 14 || len(lines[13]) != 0 {
		t.Fatal("independent thirteen-record control oracle")
	}
	resultRaw, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(control)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := os.OpenFile(control+".complete-retained", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := retained.Write(complete)
	if err := errors.Join(writeErr, retained.Sync(), retained.Close()); err != nil {
		t.Fatal("retain complete fixture evidence", err)
	}
	prefix := bytes.Join(lines[:count], nil)
	if count < 13 {
		file, err := os.OpenFile(control, os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := file.Write(prefix)
		if err := errors.Join(writeErr, file.Sync(), file.Close()); err != nil {
			t.Fatal("publish exact native control prefix", err)
		}
	}
	controlInfo, err := os.Lstat(control)
	if err != nil || !os.SameFile(before, controlInfo) || controlInfo.Size() != int64(len(prefix)) {
		t.Fatal("original control inode and exact native prefix size required", err)
	}
	resultInfo, err := os.Lstat(result)
	if err != nil {
		t.Fatal(err)
	}
	originalInfo, err := os.Lstat(boundary.Journal)
	if err != nil {
		t.Fatal(err)
	}
	_, decoded := decodeSourceCutResult(prefix, resultRaw, original, controlInfo, resultInfo, originalInfo, boundary.Journal, "initial", boundary.Generation, boundary.BindingSHA256, program)
	if count < 13 && !errors.Is(decoded, ErrBinding) || count == 13 && decoded != nil {
		t.Fatal("actual native control prefix grammar result differs", count, decoded)
	}
	files := map[string]bool{boundary.Journal: true, control: true, result: true}
	watched := map[string]bool{}
	for path := range files {
		watched[path] = true
		for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
			watched[parent] = true
			if parent == "/" {
				break
			}
		}
	}
	descriptors := sourceHistoryDescriptors(t, watched)
	for range 2 {
		reader, err := openSourceRecoveryPrefix(ctx, boundary.Journal, "initial")
		if err != nil {
			t.Fatal("fresh independent original prefix", err)
		}
		observed := reader.readOriginalCut(boundary.Generation, boundary.BindingSHA256, program)
		closed := reader.close()
		if count < 13 && (!errors.Is(observed, ErrBinding) || !sourcePhysicalRefusalClosed(observed, closed)) || count == 13 && (observed != nil || closed != nil) {
			t.Fatal("fresh proof reader prefix result differs", count, observed, closed)
		}
		if actual := sourceHistoryDescriptors(t, watched); !reflect.DeepEqual(actual, descriptors) {
			t.Fatal("control prefix reader retained descriptors before GC", count, descriptors, actual)
		}
		for path := range files {
			probe, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			locked := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
			if err := errors.Join(locked, probe.Close()); err != nil {
				t.Fatal("control prefix reader retained exclusion", count, path, err)
			}
		}
	}
	for path, expected := range map[string][]byte{boundary.Journal: original, control: prefix, result: resultRaw, control + ".complete-retained": complete} {
		actual, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatal("prefix refusal changed retained fixture evidence", count, path, err)
		}
	}
	if _, err := os.Lstat(pending); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("completed physical proof publication lost guard state", count, err)
	}
	t.Log("actual original-cut canonical prefix", count, "complete result retained; native size matched; repeated fresh readers closed before GC")
}
