package endpoint

import (
	"context"
	"time"

	requestinput "github.com/dianabuilds/ardents-network/internal/successor/installation/request"
)

// Request retains checked declarations and optional opaque original-file
// provenance. It grants no Release, native effect or runtime authority.
type Request struct {
	declared *installationRequest
	custody  *requestinput.Origin
}

type installationRequest = requestinput.Declaration

var ErrNativeUnavailable = requestinput.ErrNativeUnavailable

// DecodeRequest admits canonical portable declarations without native custody.
func DecodeRequest(ctx context.Context, raw []byte, initial bool) (Request, error) {
	document, err := requestinput.Decode(ctx, raw, initial)
	if err != nil {
		return Request{}, err
	}
	return retainRequest(document), nil
}

// ReadOwnedRequest retains checked input and opaque original native provenance.
func ReadOwnedRequest(ctx context.Context, filename string, initial bool) (Request, error) {
	document, err := requestinput.ReadOwned(ctx, filename, initial)
	if err != nil {
		return Request{}, err
	}
	return retainRequest(document), nil
}

func retainRequest(document requestinput.Document) Request {
	declared, ok := document.Declaration()
	if !ok {
		return Request{}
	}
	return Request{declared: &declared, custody: document.Origin()}
}

func (r Request) BundleRoot() string {
	if r.declared == nil {
		return ""
	}
	return r.declared.BundleRoot
}

func (r Request) ManifestSHA256() string {
	if r.declared == nil {
		return ""
	}
	return r.declared.ManifestSHA256
}

func (r Request) ReleaseHistoryRoot() string {
	if r.declared == nil {
		return ""
	}
	return r.declared.ReleaseFloorRoot
}

func (r Request) ReferenceTime() time.Time {
	if r.declared == nil {
		return time.Time{}
	}
	at, _ := time.Parse(time.RFC3339Nano, r.declared.ReferenceTime)
	return at
}

func decodeInstallationRequest(raw []byte) (installationRequest, error) {
	return requestinput.DecodeDeclaration(raw)
}

func mutableRoots(plan requestinput.Headless) []string {
	return requestinput.MutableRoots(plan)
}
