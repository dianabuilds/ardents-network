package main

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/session"
	"io"
)

func runExecutionPreparation(ctx context.Context, args []string, input io.ReadCloser, out, diagnostic io.Writer) int {
	return runExecutionHolder(ctx, args, input, out, diagnostic, false)
}
func runExecutionRouteHolder(ctx context.Context, args []string, input io.ReadCloser, out, diagnostic io.Writer) int {
	return runExecutionHolder(ctx, args, input, out, diagnostic, true)
}
func runExecutionHolder(ctx context.Context, args []string, input io.ReadCloser, out, diagnostic io.Writer, live bool) int {
	var config struct {
		Holder     holderPlan `json:"holder"`
		Generation [32]byte   `json:"generation"`
		Principal  [32]byte   `json:"principal"`
		Snapshot   string     `json:"snapshot,omitempty"`
	}
	if ctx == nil || admissionConfig(args, &config) != nil {
		return 2
	}
	return session.RunExecutionHolder(ctx, session.ExecutionHolderPlan{Holder: localHolderPlan(config.Holder), Generation: config.Generation, Principal: config.Principal, Snapshot: config.Snapshot}, input, out, diagnostic, live)
}
