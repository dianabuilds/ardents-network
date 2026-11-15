package enrollment

import (
	"errors"
	"strings"
	"testing"
)

func TestDarwinInfrastructureInitialRefusesUnselectedPlatform(t *testing.T) {
	bundle, err := VerifyInfrastructure(t.Context(), "/not-opened", strings.Repeat("0", 64))
	if !errors.Is(err, ErrInput) {
		t.Fatalf("initial admission: %v", err)
	}
	if _, ok := bundle.Facts(); ok {
		t.Fatal("unsupported initial admission returned authority")
	}
}
