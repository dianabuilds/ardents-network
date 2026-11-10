package forwarding

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
)

// Regular-file component observations supply no Completed, Startup or authority.
func TestCompletedFileBytesKeepExactContentAndEOF(t *testing.T) {
	for _, size := range []int{0, 1, (32 << 10) - 1, 32 << 10, (32 << 10) + 1, (64 << 10) - 1, 64 << 10, (64 << 10) + 1, (256 << 10) - 1, 256 << 10, (256 << 10) + 1} {
		body := bytes.Repeat([]byte{0x37}, size)
		for _, chunk := range []int{32 << 10, 64 << 10, 256 << 10} {
			file, err := os.CreateTemp(t.TempDir(), "original")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = file.Close() })
			if _, err := file.Write(body); err != nil {
				t.Fatal(err)
			}
			owner := &completedFiles{ctx: t.Context()}
			original := &initialFile{file: file, body: body}
			scratch := make([]byte, chunk)
			if err := owner.checkFileBytes(original, scratch); err != nil {
				t.Fatalf("exact size=%d chunk=%d: %v", size, chunk, err)
			}
			if _, err := file.WriteAt([]byte{0x38}, int64(size)); err != nil {
				t.Fatal(err)
			}
			if err := owner.checkFileBytes(original, scratch); !errors.Is(err, ErrBinding) {
				t.Fatalf("appended byte size=%d chunk=%d accepted: %v", size, chunk, err)
			}
			if size != 0 {
				if err := file.Truncate(int64(size - 1)); err != nil {
					t.Fatal(err)
				}
				if err := owner.checkFileBytes(original, scratch); !errors.Is(err, ErrBinding) {
					t.Fatalf("truncation size=%d chunk=%d accepted: %v", size, chunk, err)
				}
			}
		}
	}
}

func TestCompletedFileBytesRejectInPlaceChangesAcrossReadBoundaries(t *testing.T) {
	body := bytes.Repeat([]byte{0x53}, (512<<10)+1)
	file, err := os.CreateTemp(t.TempDir(), "original")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.Write(body); err != nil {
		t.Fatal(err)
	}
	owner := &completedFiles{ctx: t.Context()}
	original := &initialFile{file: file, body: body}
	for _, chunk := range []int{32 << 10, 64 << 10, 256 << 10} {
		for _, offset := range []int{0, (32 << 10) - 1, 32 << 10, (64 << 10) - 1, 64 << 10, (256 << 10) - 1, 256 << 10, len(body) - 1} {
			if _, err := file.WriteAt([]byte{0x54}, int64(offset)); err != nil {
				t.Fatal(err)
			}
			if err := owner.checkFileBytes(original, make([]byte, chunk)); !errors.Is(err, ErrBinding) {
				t.Fatalf("changed byte offset=%d chunk=%d accepted: %v", offset, chunk, err)
			}
			if _, err := file.WriteAt(body[offset:offset+1], int64(offset)); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestCompletedFileBytesRetainDenialAndPhysicalReadErrors(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "original")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	original := &initialFile{file: file, body: []byte{1}}
	if err := (&completedFiles{ctx: ctx}).checkFileBytes(original, make([]byte, 256<<10)); !errors.Is(err, context.Canceled) || errors.Is(err, os.ErrClosed) {
		t.Fatal("canceled original performed physical read", err)
	}
	err = (&completedFiles{ctx: t.Context()}).checkFileBytes(original, make([]byte, 256<<10))
	if !errors.Is(err, os.ErrClosed) || !errors.Is(err, ErrBinding) {
		t.Fatal("physical read failure lost", err)
	}
	if err := (&completedFiles{ctx: t.Context()}).checkFileBytes(original, nil); !errors.Is(err, ErrInput) {
		t.Fatal("empty comparison buffer accepted", err)
	}
}

func BenchmarkCompletedFileBytes(b *testing.B) {
	body := bytes.Repeat([]byte{0x67}, 32<<20)
	file, err := os.CreateTemp(b.TempDir(), "program")
	if err != nil {
		b.Fatal(err)
	}
	defer file.Close()
	if _, err := file.Write(body); err != nil {
		b.Fatal(err)
	}
	owner := &completedFiles{ctx: b.Context()}
	original := &initialFile{file: file, body: body}
	for _, name := range []string{"32KiB", "64KiB", "256KiB", "1MiB"} {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(body)))
			b.ReportAllocs()
			for b.Loop() {
				var err error
				switch name {
				case "32KiB":
					var scratch [32 << 10]byte
					err = owner.checkFileBytes(original, scratch[:])
					clear(scratch[:])
				case "64KiB":
					var scratch [64 << 10]byte
					err = owner.checkFileBytes(original, scratch[:])
					clear(scratch[:])
				case "256KiB":
					var scratch [256 << 10]byte
					err = owner.checkFileBytes(original, scratch[:])
					clear(scratch[:])
				case "1MiB":
					var scratch [1 << 20]byte
					err = owner.checkFileBytes(original, scratch[:])
					clear(scratch[:])
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
