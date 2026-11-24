//go:build linux || darwin

package serving

import (
	"context"
	"errors"
	"os"

	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// AcquireInputs retains the original serving declaration and its declared
// credentials. On failure it joins every already opened input.
func AcquireInputs(ctx context.Context, path string, observe func(string, *os.File)) (_ artifacts.ObservedInputs, plan servingdeclaration.ServingDeclaration, result error) {
	if ctx == nil || ctx.Err() != nil {
		return artifacts.ObservedInputs{}, plan, artifacts.ErrObservedInput
	}
	inputs := artifacts.NewObservedInputs(ctx, observe)
	defer func() {
		if result != nil {
			result = errors.Join(result, inputs.Close())
		}
	}()
	first, err := inputs.Acquire(path, 16<<10, false)
	if err != nil {
		return artifacts.ObservedInputs{}, plan, err
	}
	plan, err = servingdeclaration.DecodeServingDeclaration(first.Bytes())
	if err != nil || !ValidDeclaration(plan) {
		return artifacts.ObservedInputs{}, plan, artifacts.ErrObservedInput
	}
	for i, path := range []string{plan.Certificate, plan.PrivateKey, plan.ClientRoot} {
		bound := int64(64 << 10)
		if i == 1 {
			bound = 16 << 10
		}
		if _, err = inputs.Acquire(path, bound, i == 1); err != nil {
			return artifacts.ObservedInputs{}, plan, err
		}
	}
	if err := inputs.Check(); err != nil {
		return artifacts.ObservedInputs{}, plan, err
	}
	inputs.Watch()
	return inputs, plan, nil
}
