package custody

import (
	"context"
	"errors"
)

type sequenceSecrets struct {
	values  [][]byte
	confirm bool
}

func (input *sequenceSecrets) ReadSecret(ctx context.Context, _ SecretPrompt) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(input.values) == 0 {
		return nil, errors.New("unexpected secret prompt")
	}
	value := append([]byte(nil), input.values[0]...)
	input.values = input.values[1:]
	return value, nil
}

func (input *sequenceSecrets) Confirm(ctx context.Context, _ ConfirmationPrompt) (bool, error) {
	return input.confirm, ctx.Err()
}
