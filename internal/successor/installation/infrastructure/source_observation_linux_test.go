package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// These are physical-custody mechanism controls, not constructed Enrollment or
// Release authority and not a native successful preflight. Native command cases
// separately obtain real provenance, signatures and original manager facts.
func TestSourceInputCopiesRetainFailureAndIndependentClose(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	f, err := os.Create(filepath.Join(t.TempDir(), "original"))
	if err != nil {
		t.Fatal(err)
	}
	p := &sourceInputs{ctx: ctx, files: []*sourceFile{{file: f, raw: []byte("private")}}}
	s := SourceInputs{original: p}
	copy := s
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err = s.Check(); !errors.Is(err, context.Canceled) {
		t.Fatal("foreign copied lifetime lost original caller", err)
	}
	closed := copy.Close()
	if !errors.Is(closed, os.ErrClosed) || !errors.Is(closed, context.Canceled) {
		t.Fatal("first failure or independent physical Close failure disappeared", closed)
	}
	if !errors.Is(s.Close(), os.ErrClosed) || s.Check() == nil || copy.Check() == nil {
		t.Fatal("repeat/copy/late use revived custody")
	}
	for _, b := range p.files[0].raw {
		if b != 0 {
			t.Fatal("private bytes retained after physical close")
		}
	}
}

func TestSourceInputCloseWaitsForOriginalBorrow(t *testing.T) {
	p := &sourceInputs{ctx: t.Context()}
	s := SourceInputs{original: p}
	if err := p.begin(); err != nil {
		t.Fatal(err)
	}
	returned := make(chan error, 1)
	go func() { returned <- s.Close() }()
	// Observe synchronous denial rather than using an elapsed timeout as proof
	// of completion. Close must still wait for the original physical borrow.
	for {
		p.mu.Lock()
		closed := p.closed
		p.mu.Unlock()
		if closed {
			break
		}
		runtime.Gosched()
	}
	if s.Check() == nil {
		t.Fatal("closing lifetime admitted a new user")
	}
	select {
	case <-returned:
		t.Fatal("Close released while original user remained")
	default:
	}
	p.uses.Done()
	if err := <-returned; err != nil {
		t.Fatal(err)
	}
}
