//go:build linux

package endpoint

import "github.com/dianabuilds/ardents-network/internal/endpoint/publication"

// publicationOwner is the single local owner of one context's publication
// state: the Registration pair lifecycle, the refresh scheduler identity, the
// installed Publisher startup/drain barriers, and the two fixed failure
// reporting callbacks. Like every Context owner, the pair state carries no
// mutex of its own: its ...Locked methods run under dutyContext.mu. Only the
// refresh lifecycle keeps a private mutex for scheduler identity, which now
// lives in the publication subpackage.
//
// The owner type is named publicationOwner rather than publication so the root
// can import the publication subpackage (the refresh scheduler mechanism)
// without a package-name collision; the dutyContextState field stays
// `publication`, so every owner.publication selector is unchanged.
type publicationOwner struct {
	pair              publicationPairLifecycle
	refresh           publication.RefreshLifecycle
	starting          bool
	drain             chan struct{}
	refreshFailure    func(string)
	withdrawalFailure func(string)
}

// beginStartLocked claims the startup barrier for one installed Publisher run.
// It refuses while a start is already in flight, the pair is draining, or a
// registration is already current; on success it opens the drain channel the
// run's producers will watch. The caller holds dutyContext.mu.
func (publication *publicationOwner) beginStartLocked() bool {
	if publication.starting || publication.pair.drainingLocked() || publication.pair.currentLocked() != nil {
		return false
	}
	publication.starting = true
	publication.drain = make(chan struct{})
	return true
}

// endStartLocked releases the startup barrier. The drain channel stays open
// for the retained run's producers until withdrawal or retirement closes it.
func (publication *publicationOwner) endStartLocked() {
	publication.starting = false
}

// beginDrainLocked performs the explicit admission stop: the pair flips to
// draining first, the drain barrier closes next, and registration change
// waiters plus the refresh scheduler are signalled last. The caller holds
// dutyContext.mu and has already verified that no drain is in progress.
func (publication *publicationOwner) beginDrainLocked() {
	publication.pair.beginDrainLocked()
	close(publication.drain)
	publication.signalRegistrationsLocked()
}

// signalRegistrationsLocked wakes registration change waiters and the refresh
// scheduler in one transition. The caller holds dutyContext.mu.
func (publication *publicationOwner) signalRegistrationsLocked() {
	publication.pair.signalLocked()
	publication.refresh.Wake()
}

// stopRefresh terminates the refresh scheduler identity and discards its
// terminal cause; refresh failures are published by the refresh owner itself,
// and Context shutdown only joins resource cleanup.
func (publication *publicationOwner) stopRefresh() {
	_ = publication.refresh.Stop()
}
