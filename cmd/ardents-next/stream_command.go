package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
)

func runStreamCommand(ctx context.Context, args []string, diagnostic io.Writer) int {
	var err error
	switch {
	case len(args) == 2 && args[0] == "publish" && filepath.IsAbs(args[1]):
		err = publishApplicationStream(ctx, args[1])
	case len(args) == 4 && args[0] == "connect" && filepath.IsAbs(args[1]) && args[2] == "--link-file" && filepath.IsAbs(args[3]):
		err = connectApplicationStream(ctx, args[1], args[3])
	default:
		fmt.Fprintln(diagnostic, "usage: ardents stream connect APPLICATION_SOCKET --link-file PRIVATE_LINK_FILE")
		fmt.Fprintln(diagnostic, "       ardents stream publish ADMINISTRATION_SOCKET")
		return 2
	}
	if err != nil {
		fmt.Fprintln(diagnostic, "Application stream failed:", err)
		return 1
	}
	return 0
}
