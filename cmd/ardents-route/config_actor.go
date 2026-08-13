package main

import (
	"crypto/tls"
	"errors"
	"net"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func (raw actorPlan) actor() (route.Actor, func(), error) {
	switch raw.Role {
	case "client":
		return raw.client()
	case "publisher", "initiator", "introduction", "rendezvous", "responder":
		return raw.listener()
	default:
		return route.Actor{}, nil, errors.New("role plan has an invalid actor role")
	}
}

func (raw actorPlan) listener() (route.Actor, func(), error) {
	deadline, err := duration(raw.Deadline)
	if err != nil {
		return route.Actor{}, nil, err
	}
	certificate, err := tls.LoadX509KeyPair(raw.Certificate, raw.Key)
	if err != nil {
		return route.Actor{}, nil, err
	}
	actor := route.Actor{Role: raw.Role, ListenAddress: raw.Listen, Certificate: certificate, Deadline: deadline}
	actor.RawAttachment = raw.RawAttachment
	actor.AcknowledgementSocket, actor.AcknowledgementKeyFile = raw.AcknowledgementSocket, raw.AcknowledgementKey
	for _, field := range []struct {
		encoded     string
		destination []byte
	}{{raw.ManifestDigest, actor.ManifestDigest[:]}, {raw.NetworkID, actor.NetworkID[:]}, {raw.EpochDigest, actor.EpochDigest[:]},
		{raw.NodeID, actor.NodeID[:]}, {raw.UpstreamPin, actor.UpstreamPin[:]}} {
		if err := fixedHex(field.encoded, field.destination); err != nil {
			return route.Actor{}, nil, err
		}
	}
	actor.NextAddress = raw.Next
	if raw.Role == "publisher" {
		if !raw.RawAttachment {
			actor.ServiceCertificate, err = tls.LoadX509KeyPair(raw.ServiceCertificate, raw.ServiceKey)
			if err != nil {
				return route.Actor{}, nil, err
			}
		}
		if raw.Stream != "" {
			stream, dialErr := net.DialTimeout("unix", raw.Stream, deadline)
			if dialErr != nil {
				return route.Actor{}, nil, dialErr
			}
			actor.Stream = stream
			return actor, func() { _ = stream.Close() }, nil
		}
	} else {
		if err := fixedHex(raw.NextNodeID, actor.NextNodeID[:]); err != nil {
			return route.Actor{}, nil, err
		}
		if err := fixedHex(raw.NextPin, actor.NextPin[:]); err != nil {
			return route.Actor{}, nil, err
		}
	}
	return actor, nil, nil
}
