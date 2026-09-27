package node

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
)

func nodeAuthority(config runtimeConfig) authority.Source {
	return authority.Source{CurrentRoute: config.CurrentClosedRoute, CurrentProfile: config.CurrentClosedProfile}
}

func currentClosedRoute(config runtimeConfig, snapshot state.NodeDuty, now time.Time) (state.ClosedRouteView, error) {
	return nodeAuthority(config).Route(snapshot, now)
}
