//go:build !linux

package main

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
)

func startPublicationHolder(context.Context, publicationPlan, routePrefixPlan, admissionAuthority, *stock.Owner, *executionruntime.Operation) (publicationHandle, error) {
	return publicationHandle{}, errors.New("live Publisher native ownership unavailable")
}
