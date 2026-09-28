//go:build linux

package endpoint

import (
	"github.com/dianabuilds/ardents-network/internal/endpoint/tokenjournal"
	"github.com/dianabuilds/ardents-network/internal/entry"
)

// endpointDutyState belongs to the installed Ubuntu text composition. Its
// existing dutyMu and publisherMu ownership is unchanged by platform selection.
type endpointDutyState struct {
	publicationLive    bool
	publisherOwner     *dutyContext
	closedTokenRoot    string
	closedTokenJournal *tokenjournal.Journal
	dutyMu             dutyContextGuard
	dutyContexts       map[*dutyContext]struct{}
	dutyClosed         bool
	dutyErr            error
	closedState        closedEndpointState
	closedEntries      *entry.ClosedSets
	closedEntryRoot    string
	closedRoleRoot     string
}
