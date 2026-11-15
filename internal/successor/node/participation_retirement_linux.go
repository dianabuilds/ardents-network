package node

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
)

// releaseParticipation withdraws the original restriction only after Node's
// physical owners have joined successfully and the original runtime did not
// fail. An ordinary cancellation is a clean stop. Failure leaves the original
// Network restriction with its existing phase and expiry; neither presence nor
// absence proves later command-owned State and startup closes.
func releaseParticipation(producer *duty.NodeProducer, first, cleanup error) error {
	if producer == nil || cleanup != nil || first != nil && first != context.Canceled {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return producer.Close(ctx)
}
