//go:build linux

package main

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestSourceNativeOutputLossAndBackpressureAreFinite(t *testing.T) {
	for _, fault := range []string{"peer-close", "backpressure"} {
		t.Run(fault, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			defer writer.Close()
			if fault == "peer-close" {
				if err := reader.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				fd := int(writer.Fd())
				if err := syscall.SetNonblock(fd, true); err != nil {
					t.Fatal(err)
				}
				for {
					_, err := syscall.Write(fd, make([]byte, 4096))
					if errors.Is(err, syscall.EAGAIN) {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			started := time.Now()
			err = sourceReport(writer, "listening", "original-source")
			if fault == "peer-close" && !errors.Is(err, syscall.EPIPE) {
				t.Fatal("original output cause lost", err)
			}
			if fault == "backpressure" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("blocked output did not refuse", err)
			}
			if time.Since(started) > 2*time.Second {
				t.Fatal("output exceeded its finite bound")
			}
		})
	}
}
