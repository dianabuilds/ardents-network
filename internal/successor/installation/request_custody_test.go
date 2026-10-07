package installation

import (
	"context"
	"errors"
	"testing"
)

func TestOwnedRequestRefusesBeforeAcquiringCustody(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if request, err := ReadOwnedRequest(ctx, "/never-opened", true); !errors.Is(err, context.Canceled) || request.custody != nil || request.declared != nil {
		t.Fatalf("original cancellation lost: %v", err)
	}
	for _, filename := range []string{"", "relative", "/directory/../request"} {
		if request, err := ReadOwnedRequest(t.Context(), filename, true); err == nil || request.custody != nil || request.declared != nil {
			t.Fatalf("invalid native input acquired custody: %v", err)
		}
	}
	request, err := DecodeRequest(t.Context(), requestFixture(), true)
	if err != nil || request.custody != nil {
		t.Fatal("portable declarations became native provenance")
	}
}
