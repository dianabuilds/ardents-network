//go:build !linux

package publisher

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/endpoint/funding"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
)

func Open(context.Context, Plan, funding.Plan, funding.Authority, *stock.Owner, *executionruntime.Operation) (Publication, error) {
	return Publication{}, errors.New("live Publisher native ownership unavailable")
}
