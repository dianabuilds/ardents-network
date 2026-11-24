package main

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type routePrefixPlan struct {
	EntryRoot             string          `json:"entry_root"`
	InteriorRoot          string          `json:"interior_root"`
	SourceInteriorRoot    string          `json:"source_interior_root,omitempty"`
	ResponderInteriorRoot string          `json:"responder_interior_root,omitempty"`
	HostingRoot           string          `json:"hosting_root"`
	Domain                uint8           `json:"domain"`
	Deadline              time.Time       `json:"deadline"`
	Work                  hosting.Traffic `json:"work"`
	Termination           hosting.Traffic `json:"termination"`
	Exclusions            []route.Member  `json:"exclusions,omitempty"`
}

type routeRecipient = session.LocalRecipient
type routeJoinIntent = session.LocalJoinIntent
type routeCapsuleIntent = session.LocalCapsuleIntent
