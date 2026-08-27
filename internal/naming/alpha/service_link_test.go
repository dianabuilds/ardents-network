package alpha_test

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

func TestAlphaServiceLinkIsASeparateCanonicalDestinationForm(t *testing.T) {
	t.Parallel()

	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatalf("ParseServiceLink: %v", err)
	}
	if got := link.Name(); got != "blog.alice" {
		t.Fatalf("link.Name() = %q, want blog.alice", got)
	}
	if got := link.String(); got != "ardents-alpha://blog.alice" {
		t.Fatalf("link.String() = %q", got)
	}
}

func TestAlphaServiceLinkRejectsCanonicalNamespaceAndNonCanonicalNames(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"ardents://blog.alice",
		"ardents-alpha://Blog.Alice",
		"ardents-alpha://blog.alice/extra",
		"https://blog.alice",
	} {
		if _, err := alpha.ParseServiceLink(raw); err == nil {
			t.Errorf("ParseServiceLink(%q) accepted a non-alpha or non-canonical link", raw)
		}
	}
}
