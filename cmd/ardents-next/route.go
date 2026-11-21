package main

import (
	"path/filepath"
	"strings"
	"time"

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

// Local candidate bytes grant no receiving or Connection authority.
type routeCapsuleIntent struct {
	Slot          [32]byte  `json:"slot"`
	DeliveryNonce [32]byte  `json:"delivery_nonce"`
	Revision      uint64    `json:"revision"`
	Expiry        time.Time `json:"expiry"`
	RecipientKey  [32]byte  `json:"recipient_key"`
	Request       []byte    `json:"request"`
}
