//go:build linux

package main

import (
	"context"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
)

// showPublishedLink owns one explicit destination presentation. The value is
// never added to ordinary diagnostics, a file, or a background runtime event.
func showPublishedLink(ctx context.Context, socket string, output io.WriteCloser) error {
	if ctx == nil || output == nil {
		return errTextInput
	}
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return withTextPresentation(bounded, output.Close, func() error {
		link, err := administration.RequestPublishedLink(bounded, socket)
		if err != nil {
			return err
		}
		_, err = io.WriteString(output, link+"\n")
		return err
	})
}
