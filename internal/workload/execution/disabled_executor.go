package execution

import (
	"context"
	"errors"
	"fmt"
)

const DisabledReason = "workload execution is disabled"

type DisabledExecutor struct{}

func NewDisabledExecutor() *DisabledExecutor {
	return &DisabledExecutor{}
}

func (*DisabledExecutor) Prepare(context.Context, Request) (PreparedWorkload, error) {
	return PreparedWorkload{}, errors.New(DisabledReason)
}

func (*DisabledExecutor) Start(context.Context, PreparedWorkload) (Instance, error) {
	return Instance{}, errors.New(DisabledReason)
}

func (*DisabledExecutor) Stop(context.Context, Instance) error {
	return errors.New(DisabledReason)
}

func (*DisabledExecutor) Inspect(context.Context, string) (Instance, error) {
	return Instance{}, fmt.Errorf("workload runtime instance not found: %s", DisabledReason)
}
