//go:build linux || darwin

package main

import (
	"context"
	"io"
)

func writeNetworkPreparationReceipt(ctx context.Context, out io.Writer, raw []byte) error {
	return writeSourceReportContext(ctx, out, raw)
}
