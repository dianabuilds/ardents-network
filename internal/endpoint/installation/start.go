package installation

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/runtimeplan"
)

// AdmitStart binds the selected generation to this installed main process before
// returning its plan. No State, permission, Instance or worker is opened here.
// It does not turn stored target observations into fresh Release authorization.
func AdmitStart(ctx context.Context, root string) (runtimeplan.DecodedHeadless, error) {
	if ctx == nil {
		return runtimeplan.DecodedHeadless{}, errors.New("installed start context is unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := observePlatform(bounded); err != nil {
		return runtimeplan.DecodedHeadless{}, err
	}
	return admitInstalledStart(bounded, root)
}
