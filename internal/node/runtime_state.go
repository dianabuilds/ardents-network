package node

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

type runtimeConfig struct {
	measurementOrigin time.Time
	hostingSample     *resource.HostingSample
	hostingUsage      resource.Sample
	host              closedHostingHandle
	hostLifetime      *closedHostingLifetime
	hostingNext       time.Time
	hostingLevel      pressureLevel
	Config
	now      func() time.Time
	probe    *probePlan
	pressure *resource.Guard
}
