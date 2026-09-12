//go:build linux

package endpoint

import "github.com/dianabuilds/ardents-network/internal/entry"

// endpointTextState belongs to the installed Ubuntu text composition. Its
// existing textMu and publisherMu ownership is unchanged by platform selection.
type endpointTextState struct {
	textPublicationLive bool
	textPublisherOwner  *textContext
	closedTokenRoot     string
	closedTokenJournal  *textTokenJournal
	textMu              textContextGuard
	textContexts        map[*textContext]struct{}
	textClosed          bool
	textErr             error
	closedState         closedEndpointState
	closedEntries       *entry.ClosedSets
	closedEntryRoot     string
	closedRoleRoot      string
}

// textPublicationOwned is called with the enclosing Endpoint's publisherMu held.
func (owner *endpointTextState) textPublicationOwned() bool { return owner.textPublisherOwner != nil }

func (owner *endpointTextState) configureTextSources(source closedEndpointState, entryRoot, roleRoot string) {
	owner.closedState = source
	owner.closedEntryRoot, owner.closedRoleRoot = entryRoot, roleRoot
}
