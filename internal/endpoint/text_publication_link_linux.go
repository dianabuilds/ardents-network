//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/interfacev1/administration"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// PublishedLink projects only the committed run's canonical destination. It
// consumes fresh Administration authority, does no network lookup and cannot
// create, retry, refresh or resurrect a publication.
func (owner *textAdministration) PublishedLink(ctx context.Context) (link string, outcome error) {
	if err := owner.authorize(ctx); err != nil {
		return "", err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	run := owner.run
	if owner.closed || owner.ending || owner.pending != nil || run == nil {
		return "", errors.New("text publication Link unavailable")
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.ending {
		return "", errors.New("text publication Link unavailable")
	}
	endpoint := owner.context.endpoint
	endpoint.publisherMu.Lock()
	defer endpoint.publisherMu.Unlock()
	current := owner.context
	current.mu.Lock()
	defer current.mu.Unlock()
	_, now, err := current.textPermissionProfileLocked()
	registration := current.registration
	if err != nil || endpoint.textPublisherOwner != current || !current.liveTextServiceJobLocked(current.job, broker.Administration) ||
		current.publicationStarting || current.publicationDraining || registration == nil || !registration.published || registration.refreshAt.IsZero() || !now.Before(registration.request.Expiry) ||
		endpoint.publications == nil || run.link.Network != endpoint.network || run.link.Target == [32]byte{} || ctx.Err() != nil {
		return "", errors.New("text publication Link unavailable")
	}
	select {
	case <-registration.channel.Done():
		return "", errors.New("text publication Link unavailable")
	default:
	}
	lease, err := endpoint.publications.AcquireAt(ctx, now)
	if err != nil {
		return "", err
	}
	defer func() {
		outcome = errors.Join(outcome, lease.Close(), ctx.Err())
		if outcome != nil {
			link = ""
		}
	}()
	credential := lease.Current().Credential
	if credential.NetworkID != run.link.Network || credential.Target != run.link.Target {
		return "", errors.New("text publication Link binding changed")
	}
	return targetlink.Encode(run.link)
}

var _ administration.PublishedLinkProvider = (*textAdministration)(nil)
