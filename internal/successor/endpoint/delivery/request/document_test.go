package request

import (
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/protectedfiles"
)

func TestDocumentDetachedDeclarationsHaveNoNativeOrigin(t *testing.T) {
	document, err := Decode(t.Context(), requestFixture(), true)
	if err != nil {
		t.Fatal(err)
	}
	declared, ok := document.Declaration()
	if !ok || document.Origin() != nil {
		t.Fatal("decoded declarations acquired native provenance")
	}
	declared.Headless.NetworkAuthorities[0] = "changed"
	declared.Source.AuthorityPublic[0] = "changed"
	declared.Source.Sources[0].Family = "changed"
	fresh, ok := document.Declaration()
	if !ok || fresh.Headless.NetworkAuthorities[0] == "changed" || fresh.Source.AuthorityPublic[0] == "changed" || fresh.Source.Sources[0].Family == "changed" {
		t.Fatal("detached declarations mutated the original document")
	}
}

func TestRequestAndProtectedFilesRetainRefusalIdentity(t *testing.T) {
	for _, pair := range [][2]error{{ErrInput, protectedfiles.ErrInput}, {ErrChanged, protectedfiles.ErrChanged}, {ErrNativeUnavailable, protectedfiles.ErrNativeUnavailable}} {
		if !errors.Is(pair[0], pair[1]) || !errors.Is(pair[1], pair[0]) {
			t.Fatal("original refusal identity changed", pair)
		}
	}
}
