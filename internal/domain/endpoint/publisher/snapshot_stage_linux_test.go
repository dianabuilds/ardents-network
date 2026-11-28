package publisher

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestSnapshotStageHandsOffOnlyReadAuthority(t *testing.T) {
	root := t.TempDir()
	directory, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()

	body := bytes.Repeat([]byte{0x00, 0x80, 0xff}, (4<<20)/3+1)
	stage, err := openSnapshotStage(directory, int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	if _, err := io.Copy(stage, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	source, err := stage.finish()
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()

	for range 2 {
		got, err := io.ReadAll(io.NewSectionReader(source, 0, int64(len(body))))
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("reader obtained %d bytes, error %v", len(got), err)
		}
	}
	if _, err := source.WriteAt([]byte{0x01}, 0); err == nil {
		t.Fatal("source retained write authority")
	}
	if _, err := stage.Write([]byte{0x01}); err == nil {
		t.Fatal("stage retained write authority")
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("stage created named entries: %v, %v", entries, err)
	}
}

func TestSnapshotStageRefusesIncompleteAndExcessInput(t *testing.T) {
	directory, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	for _, body := range [][]byte{{1}, {1, 2, 3}} {
		stage, err := openSnapshotStage(directory, 2)
		if err != nil {
			t.Fatal(err)
		}
		n, err := stage.Write(body)
		if len(body) > 2 && (n != 0 || err == nil) {
			t.Fatalf("surplus write = %d, %v", n, err)
		}
		if source, err := stage.finish(); err == nil {
			source.Close()
			t.Fatal("inexact snapshot handed off")
		}
		if err := stage.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
