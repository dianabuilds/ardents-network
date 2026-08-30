package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRunOwnsOnlyBrowserAdapterLifecycle(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "browser.json")
	state := filepath.Join(directory, "browser-entry.json")
	application := filepath.Join(directory, "endpoint.sock")
	raw, err := json.Marshal(plan{Schema: "ardents-browser-adapter-v1", ApplicationSocket: application,
		BrowserEntryStatePath: state})
	if err != nil || os.WriteFile(path, raw, 0o600) != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	var output synchronizedBuffer
	done := make(chan error, 1)
	go func() { done <- run(ctx, []string{"run", path}, &output) }()
	deadline := time.Now().Add(time.Second)
	for !bytes.Contains(output.Bytes(), []byte("browser-adapter-ready")) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !bytes.Contains(output.Bytes(), []byte("browser-adapter-ready")) {
		t.Fatal("Browser Adapter did not become ready independently of Endpoint availability")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("Browser Adapter retained native-host state after stop: %v", err)
	}
	if !bytes.Contains(output.Bytes(), []byte("browser-adapter-stopped")) {
		t.Fatalf("Browser Adapter lifecycle output = %s", output.Bytes())
	}
}

type synchronizedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (buffer *synchronizedBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.Write(data)
}

func (buffer *synchronizedBuffer) Bytes() []byte {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return append([]byte(nil), buffer.buffer.Bytes()...)
}
