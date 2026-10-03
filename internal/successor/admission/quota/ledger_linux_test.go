//go:build linux

package quota

import "github.com/dianabuilds/ardents-network/internal/successor/admission"

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func resignPermissionBatch(t *testing.T, raw []byte) {
	t.Helper()
	authority := ed25519.NewKeyFromSeed(make([]byte, 32))
	seed := make([]byte, 32)
	seed[0] = 1
	holder := ed25519.NewKeyFromSeed(seed)
	copy(raw[172:236], ed25519.Sign(authority, append([]byte("ardents-issuance-permission-v1\x00"), raw[8:172]...)))
	transcript := append([]byte("ardents-issuance-request-v1\x00"), raw[80:112]...)
	transcript = append(transcript, raw[236:277]...)
	transcript = append(transcript, raw[623:625]...)
	sum := sha256.Sum256(raw[625 : len(raw)-64])
	transcript = append(transcript, sum[:]...)
	copy(raw[len(raw)-64:], ed25519.Sign(holder, transcript))
}

func TestSeparatePermissionsAndChangedAllocation(t *testing.T) {
	l, _, _ := newTestLedger(t, 1)
	raw, f, _ := batchFixture(t, 2, 1, 1, 1)
	if got := l.Debit(t.Context(), raw, f, Admitted); got != Debited {
		t.Fatal(got)
	}
	second, _, _ := batchFixture(t, 2, 1, 1, 2)
	second[80] = 5
	resignPermissionBatch(t, second)
	if got := l.Debit(t.Context(), second, f, Admitted); got != Debited {
		t.Fatal("separate permission merged", got)
	}
	changed, _, _ := batchFixture(t, 2, 1, 1, 3)
	binary.BigEndian.PutUint32(changed[164:168], 2)
	resignPermissionBatch(t, changed)
	if got := l.Debit(t.Context(), changed, f, Admitted); got != Conflict {
		t.Fatal("same permission expanded", got)
	}
}

func newTestLedger(t *testing.T, max uint32) (*Ledger, string, LedgerBinding) {
	t.Helper()
	_, _, b := batchFixture(t, 2, 1, max, 1)
	root := filepath.Join(t.TempDir(), "ledger")
	if err := Initialize(root, b); err != nil {
		t.Fatal(err)
	}
	l, err := Open(root, b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l, root, b
}
func TestDurableDebitsRetryQuotaAndCopies(t *testing.T) {
	l, root, b := newTestLedger(t, 2)
	raw, f, _ := batchFixture(t, 2, 1, 2, 1)
	if got := l.Debit(t.Context(), raw, f, Bootstrap); got != Debited {
		t.Fatal(got)
	}
	copyHandle := *l
	if got := copyHandle.Debit(t.Context(), raw, f, Bootstrap); got != AlreadyDebited {
		t.Fatal(got)
	}
	if got := l.Debit(t.Context(), raw, f, Admitted); got != Conflict {
		t.Fatal(got)
	}
	changed, _, _ := batchFixture(t, 2, 2, 2, 1)
	if got := l.Debit(t.Context(), changed, func() admission.Facts { g := f; g.Count = 2; return g }(), Bootstrap); got != Conflict {
		t.Fatal(got)
	}
	second, _, _ := batchFixture(t, 2, 1, 2, 2)
	if got := l.Debit(t.Context(), second, f, Admitted); got != Debited {
		t.Fatal(got)
	}
	third, _, _ := batchFixture(t, 2, 1, 2, 3)
	if got := l.Debit(t.Context(), third, f, Admitted); got != Exhausted {
		t.Fatal(got)
	}
	if err := copyHandle.Close(); err != nil {
		t.Fatal(err)
	}
	if got := l.Debit(t.Context(), third, f, Admitted); got != Unavailable {
		t.Fatal(got)
	}
	reopened, err := Open(root, b)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := reopened.Debit(t.Context(), raw, f, Bootstrap); got != AlreadyDebited {
		t.Fatal(got)
	}
	if got := reopened.Debit(t.Context(), third, f, Admitted); got != Exhausted {
		t.Fatal(got)
	}
	f.Now = f.DutyNotAfter
	if got := reopened.Debit(t.Context(), raw, f, Bootstrap); got != admission.Validity {
		t.Fatal(got)
	}
}

func TestBootstrapSharedClassesAndAtomicBatch(t *testing.T) {
	l, _, _ := newTestLedger(t, 3)
	for class := uint8(1); class <= 3; class++ {
		raw, f, _ := batchFixture(t, class, 1, 3, class)
		want := Debited
		if class == 3 {
			want = Exhausted
		}
		if got := l.Debit(t.Context(), raw, f, Bootstrap); got != want {
			t.Fatalf("class %d: %s", class, got)
		}
	}
	raw, f, _ := batchFixture(t, 3, 3, 3, 4)
	if got := l.Debit(t.Context(), raw, f, Admitted); got != Debited {
		t.Fatal(got)
	}
	raw, f, _ = batchFixture(t, 3, 1, 3, 5)
	if got := l.Debit(t.Context(), raw, f, Admitted); got != Exhausted {
		t.Fatal(got)
	}
	raw, f, _ = batchFixture(t, 1, 3, 3, 6)
	if got := l.Debit(t.Context(), raw, f, Admitted); got != Exhausted {
		t.Fatal(got)
	}
	raw, f, _ = batchFixture(t, 1, 2, 3, 7)
	if got := l.Debit(t.Context(), raw, f, Admitted); got != Debited {
		t.Fatal("partial batch was debited", got)
	}
}

func TestConcurrentDebitsAndCancellation(t *testing.T) {
	l, _, _ := newTestLedger(t, 1)
	raw, f, _ := batchFixture(t, 2, 1, 1, 1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	before, _ := os.ReadFile(filepath.Join(l.state.path, "admission.journal"))
	if got := l.Debit(ctx, raw, f, Admitted); got != admission.Canceled {
		t.Fatal(got)
	}
	after, _ := os.ReadFile(filepath.Join(l.state.path, "admission.journal"))
	if string(before) != string(after) {
		t.Fatal("canceled call wrote")
	}
	var wg sync.WaitGroup
	results := make(chan admission.Outcome, 16)
	for i := 0; i < 16; i++ {
		wg.Go(func() { results <- l.Debit(t.Context(), raw, f, Admitted) })
	}
	wg.Wait()
	close(results)
	success, retry := 0, 0
	for got := range results {
		switch got {
		case Debited:
			success++
		case AlreadyDebited:
			retry++
		default:
			t.Fatal(got)
		}
	}
	if success != 1 || retry != 15 {
		t.Fatalf("%d successes %d retries", success, retry)
	}
}

func TestQuotaArithmeticDutyAndPermissionIndependence(t *testing.T) {
	// Pure capacity decisions at the same owner used by durable replay. This
	// avoids thousands of fsyncs while independently exercising the 65536 bound.
	_, _, binding := batchFixture(t, 2, 1, 65536, 1)
	s := &ledgerState{binding: binding, records: map[[32]byte]debitRecord{}, duty: map[uint64]uint64{}, used: map[quotaKey]uint64{}, permissions: map[[32]byte]permissionQuota{}}
	b := verifiedBatch{request: [32]byte{1}, permission: [32]byte{1}, commitment: [32]byte{1}, window: 3600, class: 2, count: 32, maxima: [3]uint32{65536, 65536, 65536}}
	s.duty[3600] = 65504
	if !s.capacity(b, Admitted) {
		t.Fatal("exact limit refused")
	}
	s.accept(debitRecord{b, Admitted, 3600})
	b.permission[0] = 2
	b.count = 1
	if s.capacity(b, Admitted) {
		t.Fatal("new permission expanded duty")
	}
	s.duty[3600] = 0
	s.used[quotaKey{b.permission, 3600, 2}] = 65536
	if s.capacity(b, Admitted) {
		t.Fatal("permission limit expanded")
	}
	b.class = 3
	if !s.capacity(b, Admitted) {
		t.Fatal("different class merged")
	}
	s.duty[3600] = ^uint64(0)
	if s.capacity(b, Admitted) {
		t.Fatal("overflow accepted")
	}
}

func TestStorageFailuresTerminalizeAndRetainCommit(t *testing.T) {
	for _, phase := range []string{"floor-write", "floor-rename", "floor-sync", "journal-write", "journal-sync", "journal-commit", "commit-sync", "journal-close"} {
		t.Run(phase, func(t *testing.T) {
			l, root, b := newTestLedger(t, 2)
			raw, f, _ := batchFixture(t, 2, 1, 2, 1)
			f.Now = f.Now.Add(time.Second)
			hit := false
			l.state.fault = func(at string) error {
				if at == phase {
					hit = true
					return errors.New("injected")
				}
				return nil
			}
			if got := l.Debit(t.Context(), raw, f, Admitted); got != Uncertain || !hit {
				t.Fatalf("%s hit %t", got, hit)
			}
			l.state.fault = nil
			if got := l.Debit(t.Context(), raw, f, Admitted); got != Uncertain {
				t.Fatal("owner resumed", got)
			}
			_ = l.Close()
			reopened, err := Open(root, b)
			switch phase {
			case "journal-sync", "journal-commit", "floor-rename":
				if err == nil {
					reopened.Close()
					t.Fatal("incomplete transaction reopened")
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				defer reopened.Close()
				got := reopened.Debit(t.Context(), raw, f, Admitted)
				want := Debited
				if phase == "commit-sync" || phase == "journal-close" {
					want = AlreadyDebited
				}
				if got != want {
					t.Fatalf("reopen %s want %s", got, want)
				}
			}
		})
	}
}

func TestCancellationAfterCommitAndCloseResult(t *testing.T) {
	l, root, b := newTestLedger(t, 2)
	raw, f, _ := batchFixture(t, 2, 1, 2, 1)
	ctx, cancel := context.WithCancel(t.Context())
	l.state.fault = func(phase string) error {
		if phase == "commit-sync" {
			cancel()
		}
		return nil
	}
	if got := l.Debit(ctx, raw, f, Admitted); got != Debited {
		t.Fatal(got)
	}
	failure := errors.New("close injected")
	l.state.fault = func(phase string) error {
		if phase == "close" {
			return failure
		}
		return nil
	}
	if !errors.Is(l.Close(), failure) || !errors.Is(l.Close(), failure) {
		t.Fatal("close lost result")
	}
	reopened, err := Open(root, b)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := reopened.Debit(t.Context(), raw, f, Admitted); got != AlreadyDebited {
		t.Fatal(got)
	}
}

func TestAmbiguousCommitNeedsSuccessfulReopenSync(t *testing.T) {
	for _, phase := range []string{"reopen-sync", "reopen-directory-sync"} {
		t.Run(phase, func(t *testing.T) {
			l, root, binding := newTestLedger(t, 2)
			raw, facts, _ := batchFixture(t, 2, 1, 2, 1)
			l.state.fault = func(at string) error {
				if at == "commit-sync" {
					return errors.New("commit sync failed")
				}
				return nil
			}
			if got := l.Debit(t.Context(), raw, facts, Admitted); got != Uncertain {
				t.Fatal(got)
			}
			l.state.fault = nil
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			hit := false
			opened, err := openLedger(root, binding, func(at string) error {
				if at == phase {
					hit = true
					return errors.New("reopen sync failed")
				}
				return nil
			})
			if err == nil || opened != nil || !hit {
				t.Fatalf("reopen acknowledged uncertain commit: owner %v error %v hit %t", opened, err, hit)
			}
			// Failed reopen released its lease and left the exact debit intact.
			opened, err = Open(root, binding)
			if err != nil {
				t.Fatal(err)
			}
			defer opened.Close()
			if got := opened.Debit(t.Context(), raw, facts, Admitted); got != AlreadyDebited {
				t.Fatal(got)
			}
		})
	}
}

func TestCorruptionSubstitutionAndForeignRoots(t *testing.T) {
	for _, name := range []string{"tail", "marker", "hash", "pin", "floor", "pending", "symlink", "replacement", "delete", "root"} {
		t.Run(name, func(t *testing.T) {
			l, root, b := newTestLedger(t, 2)
			raw, f, _ := batchFixture(t, 2, 1, 2, 1)
			if got := l.Debit(t.Context(), raw, f, Admitted); got != Debited {
				t.Fatal(got)
			}
			_ = l.Close()
			journal := filepath.Join(root, "admission.journal")
			switch name {
			case "tail":
				file, e := os.OpenFile(journal, os.O_APPEND|os.O_WRONLY, 0600)
				if e != nil {
					t.Fatal(e)
				}
				_, _ = file.Write([]byte{1})
				_ = file.Close()
			case "marker", "hash":
				data, _ := os.ReadFile(journal)
				offset := len(data) - 1
				if name == "hash" {
					offset--
				}
				data[offset] ^= 2
				_ = os.WriteFile(journal, data, 0600)
			case "pin", "floor":
				path := filepath.Join(root, "admission."+name)
				_ = os.WriteFile(path, []byte("bad"), 0600)
			case "pending":
				_ = os.WriteFile(filepath.Join(root, "admission.pending"), []byte("bad"), 0600)
			case "symlink":
				data, _ := os.ReadFile(journal)
				other := filepath.Join(t.TempDir(), "other")
				_ = os.WriteFile(other, data, 0600)
				_ = os.Remove(journal)
				if err := os.Symlink(other, journal); err != nil {
					t.Fatal(err)
				}
			case "replacement":
				_ = os.Chmod(journal, 0644)
			case "delete":
				_ = os.Remove(journal)
			case "root":
				_ = os.Chmod(root, 0755)
			}
			before, _ := os.ReadFile(journal)
			reopened, err := Open(root, b)
			if err == nil {
				_ = reopened.Close()
				t.Fatal("corrupt root accepted")
			}
			after, _ := os.ReadFile(journal)
			if string(before) != string(after) {
				t.Fatal("refusal modified journal")
			}
		})
	}
	root := filepath.Join(t.TempDir(), "old")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "closed-token-ledger"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, b := batchFixture(t, 2, 1, 2, 1)
	if _, err := Open(root, b); err == nil {
		t.Fatal("legacy root opened")
	}
	if err := Initialize(root, b); err == nil {
		t.Fatal("existing directory replaced")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("foreign refusal wrote files")
	}
}

func TestLiveReplacementAndRetainedTimeFloor(t *testing.T) {
	l, root, b := newTestLedger(t, 2)
	raw, f, _ := batchFixture(t, 2, 1, 2, 1)
	f.Now = f.Now.Add(time.Second)
	if got := l.Debit(t.Context(), raw, f, Admitted); got != Debited {
		t.Fatal(got)
	}
	_ = l.Close()
	reopened, err := Open(root, b)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	f.Now = f.Now.Add(-time.Second)
	if got := reopened.Debit(t.Context(), raw, f, Admitted); got != admission.Validity {
		t.Fatal(got)
	}
	f.Now = f.Now.Add(time.Second)
	path := filepath.Join(root, "admission.journal")
	data, _ := os.ReadFile(path)
	if err := os.Rename(path, path+".retained"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if got := reopened.Debit(t.Context(), raw, f, Admitted); got != Uncertain {
		t.Fatal(got)
	}
}

func TestAdmissionLeaseProcess(t *testing.T) {
	if root := os.Getenv("ARDENTS_ADMISSION_TEST_ROOT"); root != "" {
		raw, f, b := batchFixture(t, 2, 1, 2, 1)
		l, err := Open(root, b)
		if err != nil {
			t.Fatal(err)
		}
		if l.Debit(t.Context(), raw, f, Admitted) != Debited {
			t.Fatal("helper debit")
		}
		_, _ = io.WriteString(os.Stdout, "READY\n")
		_, _ = io.Copy(io.Discard, os.Stdin)
		_ = l.Close()
		return
	}
	l, root, b := newTestLedger(t, 2)
	_ = l.Close()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestAdmissionLeaseProcess$")
	cmd.Env = append(os.Environ(), "ARDENTS_ADMISSION_TEST_ROOT="+root)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() })
	scanner := bufio.NewScanner(output)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "READY" {
		t.Fatal("helper not ready", scanner.Err())
	}
	if other, err := Open(root, b); !errors.Is(err, ErrBusy) {
		if other != nil {
			other.Close()
		}
		t.Fatalf("parallel open %v", err)
	}
	if err = cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	reopened, err := Open(root, b)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	raw, f, _ := batchFixture(t, 2, 1, 2, 1)
	if got := reopened.Debit(t.Context(), raw, f, Admitted); got != AlreadyDebited {
		t.Fatal("crash refunded", got)
	}
}
