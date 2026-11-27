package logstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContinueReopensOriginalInventoryWithSessionAccounting(t *testing.T) {
	at := time.Now().UTC()
	dir := filepath.Join(t.TempDir(), "logs")
	policy := Policy{SegmentBytes: 4, MaxBytes: 8, MaxFiles: 3, SegmentAge: time.Minute, MaxAge: time.Hour}
	store, err := Open(dir, policy, at)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if next, err := store.Continue(at); next != nil || err == nil {
		t.Fatal("maintenance admitted before the original writer joined", err)
	}
	for _, record := range []string{"aaaa", "bbbb", "cccc"} {
		if n, err := store.Append("events", []byte(record), at); n != 4 || err != nil {
			t.Fatal("log append", n, err)
		}
	}
	if n, err := store.Append("events", []byte("too big"), at); n != 0 || err == nil {
		t.Fatal("oversize record accepted", n, err)
	}
	if err := store.Close(); err == nil {
		t.Fatal("original source sink failure was lost")
	}

	maintenance, err := store.Continue(at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	defer maintenance.Close()
	stats := maintenance.Stats()
	if maintenance.Policy() != policy || stats.ReceivedBytes != 19 || stats.WrittenBytes != 12 ||
		stats.LostBytes != 7 || stats.RetainedBytes != 0 || stats.ExpiredBytes != 12 || stats.Files != 1 || stats.Failed {
		t.Fatalf("continuation lost original accounting or did not reobserve the inventory: %+v", stats)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "monitor.lock" {
		t.Fatal("expired log segments remain", err, entries)
	}
}

func TestLogRetentionKeepsBudgetAcrossRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	at := time.Now().UTC()
	policy := Policy{SegmentBytes: 4, MaxBytes: 8, MaxFiles: 3, SegmentAge: time.Minute, MaxAge: time.Hour}
	store, err := Open(dir, policy, at)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"1111", "2222", "3333"} {
		if _, err := store.Append("stdout", []byte(text), at); err != nil {
			t.Fatal(err)
		}
	}
	stats := store.Stats()
	if stats.RetainedBytes != 8 || stats.Files != 3 || stats.ExpiredBytes != 4 || stats.LostBytes != 0 {
		t.Fatalf("retention budget: %+v", stats)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(dir, policy, at.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append("stdout", []byte("4444"), at.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var retained strings.Builder
	var bytes int64
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		bytes += info.Size()
		if strings.HasSuffix(entry.Name(), ".log") {
			body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			retained.Write(body)
		}
	}
	if len(entries) != 3 || bytes != 8 || retained.String() != "33334444" {
		t.Fatalf("restart reset budget: files=%d bytes=%d retained=%q", len(entries), bytes, retained.String())
	}
}

func TestLogRetentionKeepsNewerActiveStream(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	at := time.Now().UTC()
	policy := Policy{SegmentBytes: 8, MaxBytes: 16, MaxFiles: 5, SegmentAge: time.Minute, MaxAge: time.Hour}
	store, err := Open(dir, policy, at)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range [][2]string{{"stdout", "1111"}, {"stdout", "2222"}, {"stdout", "3333"}, {"stderr", "aaaa"}, {"stderr", "bbbb"}, {"stdout", "4444"}} {
		if _, err := store.Append(record[0], []byte(record[1]), at); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	stdoutFiles := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "-stdout.log") {
			stdoutFiles++
			body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != "33334444" {
				t.Fatalf("newer active stream was displaced: %q", body)
			}
		}
	}
	if stdoutFiles != 1 || len(entries) != 3 {
		t.Fatalf("unexpected extra rotation: stdout=%d files=%d", stdoutFiles, len(entries))
	}
}

func TestLogSinkFailureRemainsBoundedWhileDraining(t *testing.T) {
	at := time.Now().UTC()
	store, err := Open(filepath.Join(t.TempDir(), "logs"), Policy{SegmentBytes: 8, MaxBytes: 16, MaxFiles: 3, SegmentAge: time.Minute, MaxAge: time.Hour}, at)
	if err != nil {
		t.Fatal(err)
	}
	_, first := store.Append("stdout", []byte("too-large"), at)
	if first == nil {
		t.Fatal("oversized record accepted")
	}
	for range 2 {
		if _, err := store.Append("stdout", []byte("x"), at); err == nil {
			t.Fatal("retained sink failure vanished")
		}
	}
	closed := store.Close()
	if closed == nil || closed.Error() != first.Error() {
		t.Fatalf("retained failure grew per discarded record: %v", closed)
	}
	stats := store.Stats()
	if stats.LostBytes != 11 || stats.ReceivedBytes != 11 || stats.WrittenBytes != 0 || !stats.Failed {
		t.Fatalf("drain accounting: %+v", stats)
	}
}

func TestLogSinkRetainsOriginalDescriptorFailure(t *testing.T) {
	at := time.Now().UTC()
	store, err := Open(filepath.Join(t.TempDir(), "logs"), Policy{SegmentBytes: 16, MaxBytes: 32, MaxFiles: 3, SegmentAge: time.Minute, MaxAge: time.Hour}, at)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.Append("events", []byte("seed\n"), at); err != nil {
		t.Fatal(err)
	}
	if err := store.active["events"].file.Close(); err != nil {
		t.Fatal(err)
	}
	if n, err := store.Append("events", []byte("next\n"), at); n != 0 || !errors.Is(err, os.ErrClosed) {
		t.Fatal("original descriptor failure lost", n, err)
	}
	if err := store.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("joined store lost original descriptor failure", err)
	}
	if stats := store.Stats(); !stats.Failed || stats.WrittenBytes != 5 || stats.LostBytes != 5 {
		t.Fatalf("original descriptor accounting: %+v", stats)
	}
}

func TestLogTimeRotationAndExpiryAcrossRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	at := time.Now().UTC()
	policy := Policy{SegmentBytes: 64, MaxBytes: 128, MaxFiles: 5, SegmentAge: time.Minute, MaxAge: 2 * time.Minute}
	store, err := Open(dir, policy, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append("stdout", []byte("first"), at); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append("stdout", []byte("second"), at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := store.Stats(); got.Files != 3 || got.RetainedBytes != 11 {
		t.Fatalf("time did not rotate: %+v", got)
	}
	if err := store.Prune(at.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := store.Stats(); got.Files != 2 || got.RetainedBytes != 6 || got.ExpiredBytes != 5 || got.LostBytes != 0 {
		t.Fatalf("age is not separate from loss: %+v", got)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(dir, policy, at.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if got := store.Stats(); got.Files != 1 || got.RetainedBytes != 0 || got.ExpiredBytes != 6 {
		t.Fatalf("restart missed expired history: %+v", got)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}
func TestLogStoreRefusesForeignFilesAndSharedWriter(t *testing.T) {
	at := time.Now().UTC()
	policy := Policy{SegmentBytes: 64, MaxBytes: 128, MaxFiles: 5, SegmentAge: time.Minute, MaxAge: time.Hour}
	dir := filepath.Join(t.TempDir(), "logs")
	store, err := Open(dir, policy, at)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := Open(dir, policy, at); err == nil {
		other.Close()
		t.Fatal("second writer admitted")
	}
	if _, err := store.Append("stdout", []byte("owner"), at); err != nil {
		t.Fatalf("refusal interrupted original writer: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(foreign, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if other, err := Open(dir, policy, at.Add(time.Hour)); err == nil {
		other.Close()
		t.Fatal("foreign file admitted")
	}
	body, err := os.ReadFile(foreign)
	if err != nil || string(body) != "preserve" {
		t.Fatal("foreign file was modified")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 3 {
		t.Fatal("refused inventory pruned retained owner file")
	}
	if err := os.Remove(foreign); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, logSegmentName(50, at, "stderr"))
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if other, err := Open(dir, policy, at.Add(time.Hour)); err == nil {
		other.Close()
		t.Fatal("symlink admitted")
	}
	body, err = os.ReadFile(outside)
	if err != nil || string(body) != "outside" {
		t.Fatal("outside target was modified")
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatal("refused symlink removed")
	}
}

func TestLogRetentionRefusesSmallerSegmentLimitOnRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	at := time.Now().UTC()
	policy := Policy{SegmentBytes: 8, MaxBytes: 16, MaxFiles: 4, SegmentAge: time.Minute, MaxAge: time.Hour}
	store, err := Open(dir, policy, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append("stdout", []byte("12345678"), at); err != nil {
		t.Fatal(err)
	}
	name := store.segments[0].name
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	policy.SegmentBytes = 4
	reopened, err := Open(dir, policy, at.Add(time.Second))
	if err == nil {
		reopened.Close()
		t.Fatal("restart advertised a smaller limit while keeping an oversized segment")
	}
	body, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil || string(body) != "12345678" {
		t.Fatalf("refusal changed prior evidence: %q (%v)", body, err)
	}
}

func TestLogFilesystemObservationRequiresOpenAdmittedStore(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "logs"), Policy{SegmentBytes: 4, MaxBytes: 8, MaxFiles: 3, SegmentAge: time.Minute, MaxAge: time.Hour}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	stats := store.Stats()
	if !stats.FilesystemAvailable || stats.FilesystemTotalBytes <= 0 || stats.FilesystemFreeBytes < 0 || stats.FilesystemFreeBytes > stats.FilesystemTotalBytes {
		t.Fatalf("native filesystem observation: %+v", stats)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if closed := store.Stats(); closed.FilesystemAvailable || closed.FilesystemTotalBytes != 0 || closed.FilesystemFreeBytes != 0 {
		t.Fatalf("closed store borrowed filesystem freshness: %+v", closed)
	}
}
