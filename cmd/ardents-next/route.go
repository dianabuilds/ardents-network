package main

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

type routePrefixPlan struct {
	EntryRoot    string          `json:"entry_root"`
	InteriorRoot string          `json:"interior_root"`
	HostingRoot  string          `json:"hosting_root"`
	Domain       uint8           `json:"domain"`
	Deadline     time.Time       `json:"deadline"`
	Work         hosting.Traffic `json:"work"`
	Termination  hosting.Traffic `json:"termination"`
	Exclusions   []route.Member  `json:"exclusions,omitempty"`
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
	close    func() error
	done     <-chan struct{}
	register func(context.Context, uint64) (routeRegistration, error)
}

type routeRegistration struct {
	close    func() error
	withdraw func(context.Context) error
	done     <-chan struct{}
	slot     [32]byte
}
