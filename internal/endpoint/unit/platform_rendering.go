package unit

import (
	"bytes"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

func platformCheckArgs(program, manifest, pin string) []string {
	return []string{program, "platform", "check", manifest, pin}
}

// RenderPlatform adds the exact privileged check to the enrolled unit body.
func RenderPlatform(base []byte, program, manifest, pin string) ([]byte, error) {
	if !pathgrammar.LiteralPOSIXPath(program) || !pathgrammar.LiteralPOSIXPath(manifest) ||
		!canonicaldigest.Valid(pin) || bytes.Contains(base, []byte("ExecStartPre=")) {
		return nil, ErrBinding
	}
	pre := "ExecStartPre=+:" + strings.Join(platformCheckArgs(program, manifest, pin), " ") + "\n"
	rendered := bytes.Replace(base, []byte("ExecStart="), []byte(pre+"ExecStart="), 1)
	if bytes.Equal(rendered, base) {
		return nil, ErrBinding
	}
	return rendered, nil
}
