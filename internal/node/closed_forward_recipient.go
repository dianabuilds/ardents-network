package node

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/forwarding"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func closedForwardRecipient(config runtimeConfig, snapshot state.NodeDuty, open route.ClosedOpen, now time.Time) (state.NodeDutyCandidate, error) {
	return forwarding.Recipient(nodeAuthority(config), snapshot, open, now, literalNodeEndpoint)
}
