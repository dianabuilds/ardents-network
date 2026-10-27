package cgroup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestForwardingScopeCancellationAcquiresNoKernelCustody(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	s, err := retainForwardingScope(ctx, 0, true)
	if s != nil || err != context.Canceled {
		_ = s.Close()
		t.Fatal("cancelled Forwarding acquisition retained kernel custody", err)
	}
}

func TestFixedProcessScopeDoesNotAdmitAnotherRole(t *testing.T) {
	for _, unit := range []string{"", "ardents-issuer.service", "ardents-join.service", "ardents-endpoint.service", "../ardents-forwarding.service"} {
		s, err := retainProcessScope(t.Context(), uint32(os.Getpid()), true, unit)
		if s != nil || !errors.Is(err, errInput) {
			if s != nil {
				_ = s.close()
			}
			t.Fatal("closed physical scope inventory accepted foreign unit", unit, err)
		}
	}
}

func TestForwardingScopeOrdinaryEmptyFileCannotProveJoin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events")
	if err := os.WriteFile(path, []byte("populated 0\nfrozen 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	s := &ForwardingScope{original: &processScope{ctx: t.Context(), pid: uint32(os.Getpid()), kernel: &pin{events: f, identity: info}}}
	defer s.Close()
	copy := *s
	if joined, err := s.Joined(); joined || err == nil {
		t.Fatal("ordinary empty file impersonated original Forwarding join", joined, err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	closed := s.Close()
	if !errors.Is(closed, os.ErrClosed) || copy.Close() != closed {
		t.Fatal("copied Forwarding observation erased physical Close", closed)
	}
	if joined, err := copy.Joined(); joined || err == nil {
		t.Fatal("closed original scope revived", joined, err)
	}
}
