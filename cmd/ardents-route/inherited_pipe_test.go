package main

import (
	"context"
	"os"
	"strconv"
	"testing"
)

func TestInheritedPipeReadsOneBoundedFrame(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	opened, err := openInheritedPipe(strconv.FormatUint(uint64(reader.Fd()), 10))
	if err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = writer.Write([]byte("frame")); _ = writer.Close() }()
	raw, err := readInheritedPipe(context.Background(), opened, 5)
	if err != nil || string(raw) != "frame" {
		t.Fatalf("readInheritedPipe() = %q, %v", raw, err)
	}
}
