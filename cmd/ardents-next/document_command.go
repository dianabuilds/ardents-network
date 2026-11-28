package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	appconsole "github.com/dianabuilds/ardents-network/internal/application/console"
	"github.com/dianabuilds/ardents-network/internal/application/document/localread"
)

var errDocumentUsage = errors.New("invalid document command arguments")

func runDocumentCommand(ctx context.Context, arguments []string, diagnostic io.Writer) int {
	if err := executeDocumentCommand(ctx, arguments); err != nil {
		fmt.Fprintln(diagnostic, documentFailure(err))
		return 2
	}
	return 0
}

func executeDocumentCommand(ctx context.Context, arguments []string) error {
	switch {
	case len(arguments) == 2 && arguments[0] == "link" && filepath.IsAbs(arguments[1]):
		return administration.PresentLocalLink(ctx, arguments[1])
	case len(arguments) == 3 && arguments[0] == "publish" && filepath.IsAbs(arguments[1]) && filepath.IsAbs(arguments[2]):
		return administration.PublishLocal(ctx, arguments[1], arguments[2])
	case len(arguments) == 2 && arguments[0] == "read" && filepath.IsAbs(arguments[1]):
		input, output, err := appconsole.Open()
		if err != nil {
			return err
		}
		return localread.Run(ctx, arguments[1], input, output)
	default:
		return errDocumentUsage
	}
}

func documentFailure(err error) string {
	if errors.Is(err, errDocumentUsage) {
		return participantUsage
	}
	return localread.Failure(err)
}
