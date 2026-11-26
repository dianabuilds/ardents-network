//go:build linux || darwin

package provisioning

import (
	"encoding/hex"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"strings"
	"testing"
)

func TestControlPublic32RejectsZeroAndNonCanonicalIdentifiers(t *testing.T) {
	canonical := strings.Repeat("0a", 32)
	value, err := controlPublic32(canonical)
	if err != nil || hex.EncodeToString(value[:]) != canonical {
		t.Fatal("canonical public identifier refused")
	}
	for _, raw := range []string{
		strings.Repeat("00", 32),
		strings.Repeat("0A", 32),
		strings.Repeat("0a", 31),
		strings.Repeat("0a", 33),
		"",
	} {
		value, err := controlPublic32(raw)
		if !errors.Is(err, artifacts.ErrInput) || value != [32]byte{} {
			t.Fatal("invalid public identifier classification changed", raw)
		}
	}
}
