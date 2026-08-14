package routeplan

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func (raw actorPlan) actor(waitForClientStream bool) (route.Actor, func() error, error) {
	switch raw.Role {
	case "client":
		return raw.client(waitForClientStream)
	case "publisher", "initiator", "introduction", "rendezvous", "responder":
		return raw.listener()
	default:
		return route.Actor{}, nil, errors.New("role plan has an invalid actor role")
	}
}

func (raw actorPlan) listener() (route.Actor, func() error, error) {
	deadline, err := duration(raw.Deadline)
	if err != nil {
		return route.Actor{}, nil, fmt.Errorf("parse %s Route deadline: %w", raw.Role, err)
	}
	certificate, err := tls.LoadX509KeyPair(raw.Certificate, raw.Key)
	if err != nil {
		return route.Actor{}, nil, fmt.Errorf("load %s Route certificate: %w", raw.Role, err)
	}
	lifetime, err := routeLifetime(raw.Lifetime, deadline)
	if err != nil {
		return route.Actor{}, nil, fmt.Errorf("parse %s Route lifetime: %w", raw.Role, err)
	}
	actor := route.Actor{Role: raw.Role, ListenAddress: raw.Listen, Certificate: certificate,
		Deadline: deadline, Lifetime: lifetime}
	actor.RawAttachment = raw.RawAttachment
	actor.AcknowledgementSocket, actor.AcknowledgementKeyFile = raw.AcknowledgementSocket, raw.AcknowledgementKey
	actor.IntroductionSetupSocket = raw.IntroductionSetupSocket
	actor.IntroductionForwardSocket = raw.IntroductionForwardSocket
	if raw.IntroductionSetupPeer != "" {
		if err := fixedHex(raw.IntroductionSetupPeer, actor.IntroductionSetupPeer[:]); err != nil {
			return route.Actor{}, nil, fmt.Errorf("decode %s Introduction setup peer: %w", raw.Role, err)
		}
	}
	if raw.IntroductionForwardPublic != "" {
		if err := fixedHex(raw.IntroductionForwardPublic, actor.IntroductionForwardPublic[:]); err != nil {
			return route.Actor{}, nil, fmt.Errorf("decode Introduction forward public key: %w", err)
		}
	}
	if raw.IntroductionSetupNode != "" {
		if err := fixedHex(raw.IntroductionSetupNode, actor.IntroductionSetupNode[:]); err != nil {
			return route.Actor{}, nil, fmt.Errorf("decode sealed setup Introduction Node: %w", err)
		}
	}
	for _, field := range []struct {
		name        string
		encoded     string
		destination []byte
	}{{"manifest", raw.ManifestDigest, actor.ManifestDigest[:]}, {"network", raw.NetworkID, actor.NetworkID[:]},
		{"epoch", raw.EpochDigest, actor.EpochDigest[:]}, {"Node", raw.NodeID, actor.NodeID[:]},
		{"upstream pin", raw.UpstreamPin, actor.UpstreamPin[:]}} {
		if err := fixedHex(field.encoded, field.destination); err != nil {
			return route.Actor{}, nil, fmt.Errorf("decode %s %s: %w", raw.Role, field.name, err)
		}
	}
	actor.NextAddress = raw.Next
	if raw.Role == "publisher" {
		if !raw.RawAttachment || raw.IntroductionSetupSocket != "" {
			actor.ServiceCertificate, err = tls.LoadX509KeyPair(raw.ServiceCertificate, raw.ServiceKey)
			if err != nil {
				return route.Actor{}, nil, fmt.Errorf("load publisher setup service certificate: %w", err)
			}
		}
		if raw.Stream != "" {
			stream, dialErr := net.DialTimeout("unix", raw.Stream, deadline)
			if dialErr != nil {
				return route.Actor{}, nil, fmt.Errorf("dial publisher Route stream: %w", dialErr)
			}
			actor.Stream = stream
			return actor, stream.Close, nil
		}
	} else {
		if err := fixedHex(raw.NextNodeID, actor.NextNodeID[:]); err != nil {
			return route.Actor{}, nil, fmt.Errorf("decode %s next Node: %w", raw.Role, err)
		}
		if err := fixedHex(raw.NextPin, actor.NextPin[:]); err != nil {
			return route.Actor{}, nil, fmt.Errorf("decode %s next pin: %w", raw.Role, err)
		}
	}
	return actor, nil, nil
}

func routeLifetime(value string, deadline time.Duration) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	lifetime, err := duration(value)
	if err != nil {
		return 0, err
	}
	if lifetime < deadline || lifetime > 30*time.Minute {
		return 0, errors.New("route Attachment lifetime is outside the frozen development bound")
	}
	return lifetime, nil
}
