package installation

import (
	"context"
	"errors"
	"testing"
)

func TestProvisionRefusesOriginalCancellationBeforeNativeEffects(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, test := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{"missing-context", nil, ErrInput},
		{"canceled", ctx, context.Canceled},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := ReadProvisionRequest(test.ctx, "/not-an-admitted-request")
			if !errors.Is(err, test.want) || request.declared != nil || request.custody != nil {
				t.Fatal("pre-effect request refusal lost", err)
			}
			result, err := ProvisionInitial(test.ctx, Request{}, Authorization{})
			if !errors.Is(err, test.want) || result != (ProvisionResult{}) {
				t.Fatal("pre-effect operation refusal lost", result, err)
			}
		})
	}
}
