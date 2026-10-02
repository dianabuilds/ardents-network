//go:build !linux

package main

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
)

func exportIssuerProfile(context.Context, string, []byte) error { return issuance.ErrUnsupported }
