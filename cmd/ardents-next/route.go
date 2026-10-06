package main

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

type routePrefixPlan struct {
	EntryRoot          string          `json:"entry_root"`
	InteriorRoot       string          `json:"interior_root"`
	SourceInteriorRoot string          `json:"source_interior_root,omitempty"`
	HostingRoot        string          `json:"hosting_root"`
	Domain             uint8           `json:"domain"`
	Deadline           time.Time       `json:"deadline"`
	Work               hosting.Traffic `json:"work"`
	Termination        hosting.Traffic `json:"termination"`
	Exclusions         []route.Member  `json:"exclusions,omitempty"`
}

func independentRouteRoots(roots ...string) bool {
	for i, root := range roots {
		if !absoluteAdmissionPath(root) {
			return false
		}
		for _, other := range roots[:i] {
			if root == other || strings.HasPrefix(root, other+string(filepath.Separator)) || strings.HasPrefix(other, root+string(filepath.Separator)) {
				return false
			}
		}
	}
	return true
}

type routeHandle struct {
	bootstrap bool
	issue     func(context.Context, uint8, [][32]byte) error
	close     func() error
	replenish func(context.Context) error
	done      <-chan struct{}
	register  func(context.Context, uint64) (routeRegistration, error)
	recipient func(uint8) (routeRecipient, error)
	join      func(context.Context, routeJoinIntent) (net.Conn, error)
}

type routeJoinContext struct {
	open  func(context.Context) (routeHandle, error)
	close func() error
}

type routeRecipient struct {
	Node       [32]byte  `json:"node"`
	Generation uint64    `json:"generation"`
	NotAfter   time.Time `json:"not_after"`
}

type routeJoinIntent struct {
	Choice        uint8     `json:"choice,omitzero"`
	Node          [32]byte  `json:"node"`
	Generation    uint64    `json:"generation"`
	Secret        [32]byte  `json:"secret"`
	Context       [32]byte  `json:"context"`
	Deadline      time.Time `json:"deadline"`
	SetupDeadline time.Time `json:"setup_deadline"`
}

type routeRegistration struct {
	close    func() error
	withdraw func(context.Context) error
	done     <-chan struct{}
	slot     [32]byte
}
