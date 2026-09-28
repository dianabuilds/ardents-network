//go:build linux

package administration

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
)

// RequestPublishedLink performs one bounded read-only local operation. It
// sends no Target or authority fields and never retries publication.
func RequestPublishedLink(ctx context.Context, path string) (string, error) {
	raw, err := requestBytes(ctx, path, func(output io.Writer) error { _, err := io.WriteString(output, "link\n"); return err }, maximumPublishedLink+8)
	if err != nil {
		return "", err
	}
	if len(raw) < 7 || string(raw[:5]) != "link\n" || int(binary.BigEndian.Uint16(raw[5:7])) != len(raw)-7 || !validPublishedLink(string(raw[7:])) {
		return "", errors.New("local published Link unavailable")
	}
	return string(raw[7:]), nil
}
