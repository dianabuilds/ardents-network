package forwarding

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCompletedFileFillBindsOriginalSizeAndEOF(t *testing.T) {
	for _, size := range []int{0, 1, 511, 512, 513, (64 << 10) - 1, 64 << 10, (64 << 10) + 1, (128 << 10) - 1, 128 << 10, (128 << 10) + 1} {
		for _, change := range []string{"same", "append", "truncate", "canceled"} {
			path := filepath.Join(t.TempDir(), "bytes")
			want := bytes.Repeat([]byte{0x6b}, size)
			if err := os.WriteFile(path, want, 0600); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			info, err := file.Stat()
			if err != nil {
				file.Close()
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			switch change {
			case "append":
				writer, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, written := writer.Write([]byte{0x6c})
				closed := writer.Close()
				if err := errors.Join(written, closed); err != nil {
					t.Fatal(err)
				}
			case "truncate":
				if size == 0 {
					file.Close()
					cancel()
					continue
				}
				if err := os.Truncate(path, int64(size-1)); err != nil {
					t.Fatal(err)
				}
			case "canceled":
				cancel()
			}
			original := &initialFile{file: file, info: info}
			owner := &completedFiles{ctx: ctx}
			raw, read := owner.fill(original, int64(size+2))
			closed := file.Close()
			cancel()
			if closed != nil {
				t.Fatal(closed)
			}
			if change == "same" {
				if read != nil || !bytes.Equal(raw, want) || len(original.body) != size {
					t.Fatalf("size %d exact: %v", size, read)
				}
			} else if change == "canceled" {
				if read != context.Canceled || raw != nil || original.body != nil {
					t.Fatalf("size %d canceled: %v", size, read)
				}
			} else if !errors.Is(read, ErrBinding) || raw != nil {
				t.Fatalf("size %d %s accepted: %v", size, change, read)
			}
			clear(original.body)
		}
	}
}

func TestCompletedFileFillRetainsFailureBetweenPhysicalReads(t *testing.T) {
	for _, change := range []string{"cancel", "truncate", "close"} {
		t.Run(change, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "image")
			want := bytes.Repeat([]byte{0x6b}, (192<<10)+1)
			if err := os.WriteFile(path, want, 0600); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			info, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			original := &initialFile{file: file, info: info}
			owner := &completedFiles{ctx: ctx}
			chunks, completed := 0, false
			owner.observe = func(phase string, actual *os.File) {
				if actual != file {
					t.Fatal("observation borrowed another descriptor")
				}
				if phase == "read" {
					completed = true
				}
				if phase != "fill-chunk" {
					return
				}
				chunks++
				if chunks != 1 {
					t.Fatal("failed original performed another physical read")
				}
				switch change {
				case "cancel":
					cancel()
				case "truncate":
					if err := os.Truncate(path, 64<<10); err != nil {
						t.Fatal(err)
					}
				case "close":
					if err := file.Close(); err != nil {
						t.Fatal(err)
					}
				}
			}
			raw, read := owner.fill(original, int64(len(want)))
			defer clear(original.body)
			if raw != nil || read == nil || completed || chunks != 1 {
				t.Fatalf("failed fill completed: chunks=%d completed=%v err=%v", chunks, completed, read)
			}
			if change == "cancel" {
				if read != context.Canceled {
					t.Fatal("original cancellation lost", read)
				}
			} else if !errors.Is(read, ErrBinding) || change == "close" && !errors.Is(read, os.ErrClosed) {
				t.Fatal("original physical failure lost", read)
			}
			if len(original.body) != len(want) || !bytes.Equal(original.body[:64<<10], want[:64<<10]) || !bytes.Equal(original.body[64<<10:], make([]byte, len(want)-(64<<10))) {
				t.Fatal("failed fill lost its original buffer or read beyond the interrupted chunk")
			}
		})
	}
}

// Actual ordinary file bytes only. No installed custody, canonical generation,
// Startup, State or process authority is constructed by this benchmark.
func BenchmarkCompletedFileFill32MiB(b *testing.B) {
	path := filepath.Join(b.TempDir(), "image")
	body := bytes.Repeat([]byte{0x6b}, 32<<20)
	if err := os.WriteFile(path, body, 0600); err != nil {
		b.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			b.Fatal(err)
		}
	}()
	info, err := file.Stat()
	if err != nil {
		b.Fatal(err)
	}
	owner := &completedFiles{ctx: b.Context()}
	original := &initialFile{file: file, info: info}
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	for b.Loop() {
		if _, err := file.Seek(0, 0); err != nil {
			b.Fatal(err)
		}
		raw, err := owner.fill(original, 64<<20)
		if err != nil || !bytes.Equal(raw, body) {
			b.Fatal("complete actual image differs", err)
		}
		clear(original.body)
		original.body = nil
	}
}
