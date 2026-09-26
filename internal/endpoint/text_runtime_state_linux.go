//go:build linux

package endpoint

import (
	"github.com/dianabuilds/ardents-network/internal/endpoint/tokenjournal"
	"github.com/dianabuilds/ardents-network/internal/entry"
)

// endpointTextState belongs to the installed Ubuntu text composition. Its
// existing textMu and publisherMu ownership is unchanged by platform selection.
type endpointTextState struct {
	textPublicationLive bool
	textPublisherOwner  *textContext
	closedTokenRoot     string
	closedTokenJournal  *tokenjournal.Journal
	textMu              textContextGuard
	textContexts        map[*textContext]struct{}
	textClosed          bool
	textErr             error
	closedState         closedEndpointState
	closedEntries       *entry.ClosedSets
	closedEntryRoot     string
	closedRoleRoot      string
}
