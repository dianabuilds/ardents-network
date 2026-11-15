package delivery

import (
	"context"

	requestinput "github.com/dianabuilds/ardents-network/internal/successor/endpoint/delivery/request"
)

// Request is the request owner's checked original document. It grants no
// Release, native effect or runtime authority.
type Request = requestinput.Document

type installationRequest = requestinput.Declaration

var ErrNativeUnavailable = requestinput.ErrNativeUnavailable

func decodeRequest(ctx context.Context, raw []byte, initial bool) (Request, error) {
	return requestinput.Decode(ctx, raw, initial)
}

func readOwnedRequest(ctx context.Context, filename string, initial bool) (Request, error) {
	return requestinput.ReadOwned(ctx, filename, initial)
}

func decodeInstallationRequest(raw []byte) (installationRequest, error) {
	return requestinput.DecodeDeclaration(raw)
}

func mutableRoots(plan requestinput.Headless) []string {
	return requestinput.MutableRoots(plan)
}
