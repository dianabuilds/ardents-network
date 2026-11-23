package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
)

func runStreamCommand(ctx context.Context, args []string, diagnostic io.Writer) int {
	if len(args) != 4 || args[0] != "connect" || !filepath.IsAbs(args[1]) || args[2] != "--link-file" || !filepath.IsAbs(args[3]) {
		fmt.Fprintln(diagnostic, "usage: ardents stream connect <absolute-application-socket> --link-file <absolute-private-link-file>")
		return 2
	}
	if err := connectApplicationStream(ctx, args[1], args[3]); err != nil {
		fmt.Fprintln(diagnostic, "Application stream failed:", err)
		return 1
	}
	return 0
}
