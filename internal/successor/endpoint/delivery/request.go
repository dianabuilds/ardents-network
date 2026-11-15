package delivery

import (
	"context"

	requestinput "github.com/dianabuilds/ardents-network/internal/successor/endpoint/delivery/request"
)

type installationRequest = requestinput.Declaration

var ErrNativeUnavailable = requestinput.ErrNativeUnavailable

func decodeRequest(ctx context.Context, raw []byte, initial bool) (requestinput.Document, error) {
	return requestinput.Decode(ctx, raw, initial)
}

func readOwnedRequest(ctx context.Context, filename string, initial bool) (requestinput.Document, error) {
	return requestinput.ReadOwned(ctx, filename, initial)
}

func decodeInstallationRequest(raw []byte) (installationRequest, error) {
	return requestinput.DecodeDeclaration(raw)
}

func mutableRoots(plan requestinput.Headless) []string {
	return requestinput.MutableRoots(plan)
}
