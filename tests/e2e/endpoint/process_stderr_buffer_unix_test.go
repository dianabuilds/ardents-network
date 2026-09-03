//go:build linux

package endpoint_test

import (
	"bytes"
	"sync"
)

// processStderrBuffer safely records a child process's stderr while the test
// concurrently waits for its lifecycle output on stdout.
type processStderrBuffer struct {
	mu     sync.Mutex
	output bytes.Buffer
}

func (stderr *processStderrBuffer) Write(input []byte) (int, error) {
	stderr.mu.Lock()
	defer stderr.mu.Unlock()
	return stderr.output.Write(input)
}

func (stderr *processStderrBuffer) String() string {
	stderr.mu.Lock()
	defer stderr.mu.Unlock()
	return stderr.output.String()
}
