package endpoint

import (
	"errors"
	"sync"
	"testing"
)

func TestStartupCopiesRetainOneTerminalLifetime(t *testing.T) {
	first := errors.New("original failed inspection")
	owner := &Startup{lifetime: &startupLifetime{result: first}}
	copy := *owner
	var joined sync.WaitGroup
	for _, handle := range []*Startup{owner, &copy} {
		joined.Go(func() {
			if !errors.Is(handle.Close(), first) {
				t.Error("lost original startup failure")
			}
		})
	}
	joined.Wait()
	if owner.Close() != copy.Close() {
		t.Fatal("copies published different terminal results")
	}
	if err := copy.Check(); !errors.Is(err, first) || !errors.Is(err, ErrBinding) {
		t.Fatal("closed startup regained admission", err)
	}
	if headless, source, err := owner.Plans(); err == nil || headless != nil || source != nil {
		t.Fatal("closed startup exported plans")
	}
	var empty Startup
	if empty.Check() == nil {
		t.Fatal("zero startup handle grants observation")
	}
}
