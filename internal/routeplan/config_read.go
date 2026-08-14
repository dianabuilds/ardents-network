package routeplan

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/planfile"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type actorPlan struct {
	Role, ManifestDigest, NetworkID, EpochDigest, NodeID                    string
	Listen, Certificate, Key, UpstreamPin                                   string
	NextNodeID, Next, NextPin, ServiceCertificate, ServiceKey               string
	Deadline, Lifetime, StateRoot, At, Seed, PublisherPin                   string
	Stream                                                                  string
	RawAttachment                                                           bool
	AcknowledgementSocket, AcknowledgementKey                               string
	IntroductionSetupSocket, IntroductionSetupPeer, IntroductionSetupPublic string
	IntroductionForwardSocket, IntroductionForwardPublic                    string
	IntroductionServicePublic, IntroductionSetupNode                        string
	Authorities, ExcludedFamilies, ExcludedDomains                          []string
	ExcludedIdentities                                                      []string
	Threshold                                                               int
	Attachments                                                             uint32
	AttachmentPlans                                                         []attachmentPlan
	ConcurrentAttachments                                                   bool
}

func (value actorPlan) validateRoleLocal() error {
	if value.Attachments > 4 || len(value.AttachmentPlans) > 4 ||
		(value.Attachments != 0 && len(value.AttachmentPlans) != 0) {
		return errors.New("route Attachment process count is outside its bound")
	}
	if value.ConcurrentAttachments && (value.Role != "publisher" || len(value.AttachmentPlans) < 2) {
		return errors.New("concurrent Route Attachments require one bounded publisher plan")
	}
	for index := range value.AttachmentPlans {
		if err := value.AttachmentPlans[index].validate(value.Role); err != nil {
			return err
		}
		if _, err := value.attachmentPlan(index); err != nil {
			return err
		}
	}
	return value.validateSingleRoleLocal()
}

func (value actorPlan) validateSingleRoleLocal() error {
	clientOnly := value.StateRoot != "" || value.At != "" || value.Seed != "" || value.PublisherPin != "" ||
		len(value.Authorities) != 0 || value.Threshold != 0 || len(value.ExcludedIdentities) != 0 ||
		len(value.ExcludedFamilies) != 0 || len(value.ExcludedDomains) != 0
	nextOnly := value.NextNodeID != "" || value.Next != "" || value.NextPin != ""
	serviceOnly := value.ServiceCertificate != "" || value.ServiceKey != ""
	listenerOnly := value.Listen != "" || value.UpstreamPin != "" || value.NodeID != "" || value.EpochDigest != ""
	switch value.Role {
	case "client":
		if listenerOnly || nextOnly || serviceOnly {
			return errors.New("client plan contains information outside its role-local duty")
		}
		if value.RawAttachment && (value.Stream == "" || value.PublisherPin != "") {
			return errors.New("raw client attachment plan is invalid")
		}
	case "publisher":
		if clientOnly || nextOnly {
			return errors.New("publisher plan contains information outside its role-local duty")
		}
		if value.RawAttachment && (value.Stream == "" || serviceOnly && value.IntroductionSetupSocket == "") {
			return errors.New("raw publisher attachment plan is invalid")
		}
	case "initiator", "introduction", "rendezvous", "responder":
		if clientOnly || serviceOnly || value.Stream != "" || value.RawAttachment {
			return errors.New("node plan contains information outside its role-local duty")
		}
	default:
		return errors.New("role plan has an invalid actor role")
	}
	if value.Role == "introduction" {
		if (value.AcknowledgementSocket == "") != (value.AcknowledgementKey == "") {
			return errors.New("introduction acknowledgement surface is incomplete")
		}
	} else if value.AcknowledgementSocket != "" || value.AcknowledgementKey != "" {
		return errors.New("non-introduction plan contains publication acknowledgement input")
	}
	if value.Role == "introduction" {
		present := value.IntroductionSetupSocket != ""
		if present != (value.IntroductionSetupPeer != "") || present != (value.IntroductionForwardSocket != "") ||
			present != (value.IntroductionForwardPublic != "") || value.IntroductionSetupPublic != "" ||
			value.IntroductionServicePublic != "" || value.IntroductionSetupNode != "" {
			return errors.New("introduction sealed setup surface is incomplete")
		}
	} else if value.Role == "client" {
		present := value.IntroductionSetupSocket != ""
		if value.IntroductionSetupPeer != "" || value.IntroductionForwardSocket != "" ||
			value.IntroductionForwardPublic != "" || value.IntroductionSetupNode != "" ||
			present != (value.IntroductionSetupPublic != "") || present != (value.IntroductionServicePublic != "") {
			return errors.New("client sealed Introduction setup is incomplete")
		}
	} else if value.Role == "publisher" {
		present := value.IntroductionSetupSocket != ""
		if value.IntroductionSetupPublic != "" || value.IntroductionForwardSocket != "" ||
			value.IntroductionForwardPublic != "" || value.IntroductionServicePublic != "" ||
			present != (value.IntroductionSetupPeer != "") || present != (value.IntroductionSetupNode != "") {
			return errors.New("publisher sealed Introduction setup service is incomplete")
		}
	} else if value.IntroductionSetupSocket != "" || value.IntroductionSetupPeer != "" ||
		value.IntroductionSetupPublic != "" || value.IntroductionForwardSocket != "" ||
		value.IntroductionForwardPublic != "" || value.IntroductionServicePublic != "" || value.IntroductionSetupNode != "" {
		return errors.New("role plan contains a sealed Introduction duty outside its role")
	}
	return nil
}

// Sequence owns the bounded actors declared for one role-local process.
type Sequence struct {
	plan actorPlan
	next int
}

// Step is one constructed role-local Attachment and its owned resources.
type Step struct {
	Actor      route.Actor
	Attachment uint32
	More       bool
	close      func() error
}

// Concurrent reports whether every bounded listener Attachment may run together.
func (value *Sequence) Concurrent() bool {
	return value != nil && value.plan.ConcurrentAttachments
}

// Close releases resources owned by this Attachment step.
func (value Step) Close() error {
	if value.close == nil {
		return nil
	}
	return value.close()
}

// Load reads and validates one bounded role-local process plan.
func Load(path string) (*Sequence, error) {
	var value actorPlan
	if err := planfile.Decode(path, 64<<10, &value); err != nil {
		return nil, err
	}
	if err := value.validateRoleLocal(); err != nil {
		return nil, err
	}
	return &Sequence{plan: value}, nil
}

// Next constructs the next bounded role-local Attachment step.
func (value *Sequence) Next() (Step, bool, error) {
	if value == nil || value.next >= value.plan.attachmentCount() {
		return Step{}, false, nil
	}
	plan, err := value.plan.attachmentPlan(value.next)
	if err != nil {
		return Step{}, false, err
	}
	actor, closeState, err := plan.actor()
	if value.clientTerminal(err) {
		value.next = value.plan.attachmentCount()
		return Step{}, false, nil
	}
	value.next++
	return Step{Actor: actor, close: closeState, Attachment: uint32(value.next),
		More: value.next < value.plan.attachmentCount()}, true, err
}

func (value *Sequence) clientTerminal(err error) bool {
	var unavailable *routeStreamUnavailable
	return value != nil && value.plan.Role == "client" && value.next > 0 && errors.As(err, &unavailable)
}

func fixedHex(encoded string, destination []byte) error {
	return planfile.FixedHex(encoded, destination)
}

func duration(value string) (time.Duration, error) {
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
