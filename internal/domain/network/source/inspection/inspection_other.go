//go:build !linux

package inspection

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
)

// errInspectionPlatformUnavailable is the explicit typed unavailable outcome
// for the authenticated Direct-Origin Source contact on platforms without the
// Linux Source preparation mechanism (ADR-0129). InspectLatest composes
// state.SourcePreparationConfig, which remains Linux-only; a missing required
// protection yields the same explicit refusal on every other platform and never
// supplies a weaker accepting path.
var errInspectionPlatformUnavailable = errors.New("source inspection platform unavailable")

// Request selects one Source and the original files used to authenticate it.
type Request struct {
	ServingPlan, ClientCertificate, ClientKey, ServerCA string
	ServerName, Family, Handle                          string
	ServerKeyPin, SourceID                              [32]byte
	MaterialIndex                                       uint32
}

// InspectLatest refuses the authenticated Source contact on platforms without
// the Linux Source preparation mechanism. It retains no input or credential and
// performs no contact; the caller observes the explicit unavailable result.
func InspectLatest(ctx context.Context, request Request) (source.Message, error) {
	return source.Message{}, errInspectionPlatformUnavailable
}
