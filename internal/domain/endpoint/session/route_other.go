//go:build !linux && !darwin

package session

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
)

func OpenPrefix(context.Context, funding.Plan, funding.Authority, *stock.Owner) (Prefix, error) {
	return Prefix{}, errors.New("route requires Linux")
}
func OpenBootstrap(context.Context, funding.Plan, funding.Authority, *stock.Owner) (Prefix, error) {
	return Prefix{}, errors.New("route selection and durable roots require Linux")
}
func OpenJoinContext(context.Context, funding.Plan, funding.Authority, *stock.Owner) (JoinContext, error) {
	return JoinContext{}, errors.New("route requires Linux")
}
