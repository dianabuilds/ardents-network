package main

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
)

func writeNetworkPreparationReceipt(ctx context.Context, out io.Writer, raw []byte) error {
	return artifacts.WriteReport(ctx, out, raw)
}
