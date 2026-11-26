//go:build linux

package administration

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
)

// PresentPublishedLink presents one authorized local Link and joins closure of
// the supplied output on completion or cancellation.
func PresentPublishedLink(ctx context.Context, socket string, output io.WriteCloser) (outcome error) {
	if ctx == nil || output == nil {
		return connection.ErrInvalidTargetLink
	}
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var closeErr error
	closed := make(chan struct{})
	stop := context.AfterFunc(bounded, func() {
		defer close(closed)
		closeErr = output.Close()
	})
	defer func() {
		if stop() {
			closeErr = output.Close()
		} else {
			<-closed
		}
		outcome = errors.Join(outcome, closeErr, bounded.Err())
	}()
	link, err := RequestPublishedLink(bounded, socket)
	if err != nil {
		return err
	}
	_, err = io.WriteString(output, link+"\n")
	return err
}

// RequestPublishedLink performs one bounded read-only local operation. It
// sends no Target or authority fields and never retries publication.
func RequestPublishedLink(ctx context.Context, path string) (string, error) {
	raw, err := exchangeLocalRequest(ctx, path, func(output io.Writer) error { _, err := io.WriteString(output, "link\n"); return err }, maximumPublishedLink+8)
	if err != nil {
		return "", err
	}
	if len(raw) < 7 || string(raw[:5]) != "link\n" || int(binary.BigEndian.Uint16(raw[5:7])) != len(raw)-7 || !validPublishedLink(string(raw[7:])) {
		return "", errors.New("local published Link unavailable")
	}
	return string(raw[7:]), nil
}
