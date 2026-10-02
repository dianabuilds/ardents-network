//go:build !linux

package main

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
)

func checkIssuanceOutput(context.Context, string) error          { return issuance.ErrUnsupported }
func exportIssuanceOutput(context.Context, string, []byte) error { return issuance.ErrUnsupported }
