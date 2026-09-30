//go:build ignore

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogRetentionKeepsBudgetAcrossRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	at := time.Now().UTC()
	policy := logRetentionPolicy{SegmentBytes: 4, MaxBytes: 8, MaxFiles: 3, SegmentAge: time.Minute, MaxAge: time.Hour}
	store, err := openLogStore(dir, policy, at)
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
	store, err = openLogStore(dir, policy, at.Add(time.Second))
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
	policy := logRetentionPolicy{SegmentBytes: 8, MaxBytes: 16, MaxFiles: 5, SegmentAge: time.Minute, MaxAge: time.Hour}
	store, err := openLogStore(dir, policy, at)
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
	store, err := openLogStore(filepath.Join(t.TempDir(), "logs"), logRetentionPolicy{SegmentBytes: 8, MaxBytes: 16, MaxFiles: 3, SegmentAge: time.Minute, MaxAge: time.Hour}, at)
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

func TestLogTimeRotationAndExpiryAcrossRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	at := time.Now().UTC()
	policy := logRetentionPolicy{SegmentBytes: 64, MaxBytes: 128, MaxFiles: 5, SegmentAge: time.Minute, MaxAge: 2 * time.Minute}
	store, err := openLogStore(dir, policy, at)
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
	store, err = openLogStore(dir, policy, at.Add(3*time.Minute))
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
	policy := logRetentionPolicy{SegmentBytes: 64, MaxBytes: 128, MaxFiles: 5, SegmentAge: time.Minute, MaxAge: time.Hour}
	dir := filepath.Join(t.TempDir(), "logs")
	store, err := openLogStore(dir, policy, at)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := openLogStore(dir, policy, at); err == nil {
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
	if other, err := openLogStore(dir, policy, at.Add(time.Hour)); err == nil {
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
	if other, err := openLogStore(dir, policy, at.Add(time.Hour)); err == nil {
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
