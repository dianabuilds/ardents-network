package node

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
)

// releaseParticipation withdraws the original restriction only after Node's
// physical owners have joined successfully. A failed close leaves the durable
// restriction in place with its original Network phase and expiry. Neither its
// absence nor its presence proves later command-owned State and startup closes.
func releaseParticipation(producer *duty.NodeProducer, cleanup error) error {
	if producer == nil || cleanup != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return producer.Close(ctx)
}
