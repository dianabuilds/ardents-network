package main

import (
	"context"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

func runAdmissionReceiver(ctx context.Context, args []string, input io.ReadCloser, out, diagnostic io.Writer) int {
	var config struct {
		Root     string                      `json:"root"`
		Profile  string                      `json:"profile"`
		Network  *state.AuthorityDeclaration `json:"network,omitempty"`
		Receiver receiving.Receiver          `json:"receiver"`
		NotAfter time.Time                   `json:"not_after"`
	}
	if ctx == nil || admissionConfig(args, &config) != nil {
		return 2
	}
	return funding.RunLocalReceiver(ctx, config.Root, config.Profile, config.Network, config.Receiver, config.NotAfter, input, out, diagnostic)
}
