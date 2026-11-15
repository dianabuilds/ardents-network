package delivery

import (
	"context"
	"errors"
	"testing"

	requestinput "github.com/dianabuilds/ardents-network/internal/successor/endpoint/delivery/request"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/release/endpoint"
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
			if !errors.Is(err, test.want) || requestHasDeclaration(request) || request.Origin() != nil {
				t.Fatal("pre-effect request refusal lost", err)
			}
			result, err := ProvisionInitial(test.ctx, requestinput.Document{}, generationauthorization.Authorization{})
			if !errors.Is(err, test.want) || result != (ProvisionResult{}) {
				t.Fatal("pre-effect operation refusal lost", result, err)
			}
		})
	}
}
