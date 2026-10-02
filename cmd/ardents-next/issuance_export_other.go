//go:build !linux

package main

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/successor/issuance"
)

func exportIssuanceInventory(context.Context, string, []byte) error { return issuance.ErrUnsupported }
