package funding

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// Declaration is the local holder's Route configuration, not a permission.
// Its field names preserve the existing holder configuration format.
type Declaration struct {
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

// Plan returns the unchanged local bounds for validation by their owners.
func (d Declaration) Plan() Plan {
	return Plan(d)
}
