//go:build ignore

package main

import (
	"bytes"
	"sync"
)

type boundedBuffer struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (b *boundedBuffer) Write(content []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - b.buffer.Len()
	if remaining > len(content) {
		remaining = len(content)
	}
	if remaining > 0 {
		_, _ = b.buffer.Write(content[:remaining])
	}
	if remaining < len(content) {
		b.exceeded = true
	}
	return len(content), nil
}

func (b *boundedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte{}, b.buffer.Bytes()...)
}

func (b *boundedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Len()
}

func (b *boundedBuffer) Exceeded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.exceeded
}
