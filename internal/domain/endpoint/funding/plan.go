package funding

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/route"
)

// Plan declares original local roots and finite opening bounds, not a CLI
// schema or a permission. Each real owner independently validates its inputs.
type Plan struct {
	EntryRoot, InteriorRoot, SourceInteriorRoot, ResponderInteriorRoot, HostingRoot string
	Domain                                                                          uint8
	Deadline                                                                        time.Time
	Work, Termination                                                               hosting.Traffic
	Exclusions                                                                      []route.Member
}
