//go:build linux

package administration

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPublishedLinkUsesOnlyExplicitLocalProjection(t *testing.T) {
	owner := &publishedLinkTestOwner{link: "ardents://bounded-public-destination"}
	owner.publish = func(context.Context) error { t.Error("Link query published"); return nil }
	owner.withdraw = func(context.Context) error { t.Error("Link query withdrew"); return nil }
	path, server := snapshotTestServer(t, owner)
	defer func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	}()
	link, err := RequestPublishedLink(t.Context(), path)
	if err != nil || link != owner.link || owner.calls.Load() != 1 {
		t.Fatalf("Link projection = %q, %v, calls%d", link, err, owner.calls.Load())
	}
}

func TestPublishedLinkRejectsUnavailableAndUnsafeProjection(t *testing.T) {
	for _, test := range []struct {
		name, link string
		failure    error
	}{
		{"empty", "", nil}, {"oversize", strings.Repeat("x", 513), nil}, {"newline", "x\ny", nil}, {"escape", "x\x1by", nil}, {"space", "x y", nil}, {"nonascii", "é", nil}, {"owner failure", "valid", errors.New("not committed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			owner := &publishedLinkTestOwner{link: test.link, failure: test.failure}
			path, server := snapshotTestServer(t, owner)
			defer func() {
				if err := server.Close(); err != nil {
					t.Error(err)
				}
			}()
			if link, err := RequestPublishedLink(t.Context(), path); err == nil || link != "" {
				t.Fatalf("invalid Link projection accepted: %q %v", link, err)
			}
		})
	}
	path, server := snapshotTestServer(t, testInterface{publish: func(context.Context) error { t.Error("unsupported Link query published"); return nil }})
	defer func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := RequestPublishedLink(t.Context(), path); err == nil {
		t.Fatal("legacy owner supplied a Link")
	}
}
