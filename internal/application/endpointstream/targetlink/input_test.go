package targetlink

import (
	"errors"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

func TestReadPreservesBoundedTargetLink(t *testing.T) {
	for _, test := range []struct {
		name, input, want string
		invalid           bool
	}{
		{"line ending", "explicit-link\r\n", "explicit-link", false},
		{"bounded maximum", strings.Repeat("x", 512), strings.Repeat("x", 512), false},
		{"empty", "\n", "", true},
		{"overlong", strings.Repeat("x", 513), "", true},
		{"invalid UTF-8", string([]byte{0xff}), "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Read(strings.NewReader(test.input))
			if test.invalid {
				if got != "" || !errors.Is(err, endpointstream.ErrInvalidLocalInput) {
					t.Fatalf("invalid input accepted: %q, %v", got, err)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("target link = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}
