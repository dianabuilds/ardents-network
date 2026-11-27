package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

func runStreamCommand(ctx context.Context, args []string, diagnostic io.Writer) int {
	publish := len(args) == 2 && args[0] == "publish" && filepath.IsAbs(args[1])
	connect := len(args) == 4 && args[0] == "connect" && filepath.IsAbs(args[1]) && args[2] == "--link-file" && filepath.IsAbs(args[3])
	if !publish && !connect {
		_, err := io.WriteString(diagnostic, "usage: ardents stream connect APPLICATION_SOCKET --link-file PRIVATE_LINK_FILE\n"+
			"       ardents stream publish ADMINISTRATION_SOCKET\n")
		return streamCommandStatus(2, err)
	}
	var err error
	if publish {
		err = endpointstream.PublishLocalStream(ctx, args[1])
	} else {
		err = endpointstream.ConnectLocalStream(ctx, args[1], args[3])
	}
	if err != nil {
		_, writeErr := fmt.Fprintln(diagnostic, "Application stream failed:", err)
		return streamCommandStatus(1, writeErr)
	}
	return 0
}

func streamCommandStatus(code int, diagnosticErr error) int {
	if diagnosticErr != nil {
		return 2
	}
	return code
}
