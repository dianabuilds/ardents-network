package routeplan

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/planfile"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type routeStreamUnavailable struct{ err error }

func (value *routeStreamUnavailable) Error() string { return value.err.Error() }
func (value *routeStreamUnavailable) Unwrap() error { return value.err }

const replacementStreamWait = 500 * time.Millisecond

func (raw actorPlan) client(initialStream bool) (route.Actor, func() error, error) {
	at, err := time.Parse(time.RFC3339, raw.At)
	if err != nil {
		return route.Actor{}, nil, fmt.Errorf("parse client selection time: %w", err)
	}
	var manifest, network, seed, publisher [32]byte
	if err := fixedHex(raw.ManifestDigest, manifest[:]); err != nil {
		return route.Actor{}, nil, fmt.Errorf("decode client manifest: %w", err)
	}
	if err := fixedHex(raw.NetworkID, network[:]); err != nil {
		return route.Actor{}, nil, fmt.Errorf("decode client Network ID: %w", err)
	}
	if err := fixedHex(raw.Seed, seed[:]); err != nil {
		return route.Actor{}, nil, fmt.Errorf("decode client selection seed: %w", err)
	}
	if !raw.RawAttachment {
		if err := fixedHex(raw.PublisherPin, publisher[:]); err != nil {
			return route.Actor{}, nil, fmt.Errorf("decode client publisher pin: %w", err)
		}
	}
	authorities, err := planfile.Authorities(raw.Authorities, 8)
	if err != nil {
		return route.Actor{}, nil, fmt.Errorf("decode client Network State authorities: %w", err)
	}
	opened, err := state.Open(state.Config{Root: raw.StateRoot, NetworkID: network,
		Authorities: authorities, Threshold: raw.Threshold, AcceptedProfile: "h3-route-tracer-v1", Now: at})
	if err != nil {
		return route.Actor{}, nil, fmt.Errorf("open client Network State: %w", err)
	}
	view, err := opened.Current()
	if err != nil {
		return route.Actor{}, nil, errors.Join(fmt.Errorf("read current client Network State: %w", err), opened.Close())
	}
	excluded, err := identities(raw.ExcludedIdentities)
	if err != nil {
		return route.Actor{}, nil, errors.Join(fmt.Errorf("decode client excluded identities: %w", err), opened.Close())
	}
	plan, err := route.Select(view, route.Selection{Seed: seed, At: at, ExcludedIdentities: excluded,
		ExcludedFamilies: raw.ExcludedFamilies, ExcludedDomains: raw.ExcludedDomains})
	if err != nil {
		return route.Actor{}, nil, errors.Join(fmt.Errorf("select client Route: %w", err), opened.Close())
	}
	certificate, err := tls.LoadX509KeyPair(raw.Certificate, raw.Key)
	if err != nil {
		return route.Actor{}, nil, errors.Join(fmt.Errorf("load client Route certificate: %w", err), opened.Close())
	}
	deadline, err := duration(raw.Deadline)
	if err != nil {
		return route.Actor{}, nil, errors.Join(fmt.Errorf("parse client Route deadline: %w", err), opened.Close())
	}
	lifetime, err := routeLifetime(raw.Lifetime, deadline)
	if err != nil {
		return route.Actor{}, nil, errors.Join(fmt.Errorf("parse client Route lifetime: %w", err), opened.Close())
	}
	actor := route.Actor{Role: "client", ManifestDigest: manifest, Plan: plan, ClientCertificate: certificate,
		PublisherPin: publisher, Deadline: deadline, Lifetime: lifetime, RawAttachment: raw.RawAttachment,
		LocalRoleStateRoot: raw.LocalRoleStateRoot}
	actor.IntroductionSetupSocket = raw.IntroductionSetupSocket
	if raw.IntroductionSetupPublic != "" {
		if err := fixedHex(raw.IntroductionSetupPublic, actor.IntroductionSetupPublic[:]); err != nil {
			return route.Actor{}, nil, errors.Join(fmt.Errorf("decode client Introduction public key: %w", err), opened.Close())
		}
	}
	if raw.IntroductionServicePublic != "" {
		if err := fixedHex(raw.IntroductionServicePublic, actor.IntroductionServicePublic[:]); err != nil {
			return route.Actor{}, nil, errors.Join(fmt.Errorf("decode sealed setup service public key: %w", err), opened.Close())
		}
	}
	if raw.Stream != "" {
		wait := replacementStreamWait
		if initialStream {
			wait = deadline
		}
		stream, dialErr := dialClientStream(raw.Stream, deadline, wait)
		if dialErr != nil {
			failure := errors.Join(fmt.Errorf("dial client Route stream: %w", dialErr), opened.Close())
			return route.Actor{}, nil, &routeStreamUnavailable{err: failure}
		}
		actor.Stream = stream
		return actor, func() error { return errors.Join(stream.Close(), opened.Close()) }, nil
	}
	return actor, opened.Close, nil
}

func dialClientStream(path string, setupDeadline, wait time.Duration) (net.Conn, error) {
	if setupDeadline <= 0 {
		return nil, errors.New("client stream setup deadline is invalid")
	}
	if wait <= 0 {
		return nil, errors.New("client stream wait is invalid")
	}
	deadline := time.Now().Add(min(setupDeadline, wait))
	var lastErr error
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		connection, err := net.DialTimeout("unix", path, min(50*time.Millisecond, remaining))
		if err == nil {
			return connection, nil
		}
		lastErr = err
		time.Sleep(min(10*time.Millisecond, max(time.Duration(0), time.Until(deadline))))
	}
	if lastErr == nil {
		return nil, errors.New("client recovery stream wait expired before dialing")
	}
	return nil, lastErr
}

func identities(values []string) ([][32]byte, error) {
	if len(values) == 0 {
		return nil, nil
	}
	return planfile.Digests(values, 64)
}
