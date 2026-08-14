package routeplan

import "errors"

// attachmentPlan contains only values that may change between bounded
// Attachments owned by one role-local Route process.
type attachmentPlan struct {
	Seed                                         string
	ExcludedIdentities                           []string
	ExcludedFamilies, ExcludedDomains            []string
	UpstreamPin, NextNodeID, Next, NextPin       string
	AcknowledgementSocket                        string
	AcknowledgementKey                           string
	IntroductionSetupSocket                      string
	IntroductionSetupPublic                      string
	IntroductionServicePublic                    string
	IntroductionSetupPeer, IntroductionSetupNode string
	Listen, ServiceCertificate, ServiceKey       string
}

func (value attachmentPlan) validate(role string) error {
	clientFields := value.Seed != "" || len(value.ExcludedIdentities) != 0 ||
		len(value.ExcludedFamilies) != 0 || len(value.ExcludedDomains) != 0
	nextFields := value.NextNodeID != "" || value.Next != "" || value.NextPin != ""
	acknowledgement := value.AcknowledgementSocket != "" || value.AcknowledgementKey != ""
	publisherOnly := value.Listen != "" || value.ServiceCertificate != "" || value.ServiceKey != "" ||
		value.IntroductionSetupPeer != "" || value.IntroductionSetupNode != ""
	introductionSetup := value.IntroductionSetupSocket != "" || value.IntroductionSetupPublic != "" ||
		value.IntroductionServicePublic != "" || value.IntroductionSetupPeer != "" || value.IntroductionSetupNode != ""
	if (value.AcknowledgementSocket == "") != (value.AcknowledgementKey == "") {
		return errors.New("attachment Introduction acknowledgement surface is incomplete")
	}
	switch role {
	case "client":
		if value.UpstreamPin != "" || nextFields || acknowledgement || publisherOnly ||
			(value.IntroductionSetupSocket == "") != (value.IntroductionSetupPublic == "") ||
			(value.IntroductionSetupSocket == "") != (value.IntroductionServicePublic == "") {
			return errors.New("client attachment plan contains Node-role information")
		}
	case "publisher":
		present := value.IntroductionSetupSocket != ""
		if clientFields || nextFields || acknowledgement || value.IntroductionSetupPublic != "" ||
			value.IntroductionServicePublic != "" || present != (value.IntroductionSetupPeer != "") ||
			present != (value.IntroductionSetupNode != "") ||
			present != (value.ServiceCertificate != "" && value.ServiceKey != "") {
			return errors.New("publisher attachment plan contains cross-role information")
		}
	case "introduction":
		if clientFields || introductionSetup || publisherOnly || value.Listen != "" {
			return errors.New("introduction attachment plan contains client information")
		}
	case "initiator", "rendezvous", "responder":
		if clientFields || acknowledgement || introductionSetup || publisherOnly || value.Listen != "" {
			return errors.New("node attachment plan contains cross-role information")
		}
	default:
		return errors.New("attachment plan has an invalid actor role")
	}
	return nil
}

func (value actorPlan) attachmentCount() int {
	if len(value.AttachmentPlans) != 0 {
		return len(value.AttachmentPlans)
	}
	if value.Attachments != 0 {
		return int(value.Attachments)
	}
	return 1
}

func (value actorPlan) attachmentPlan(index int) (actorPlan, error) {
	if index < 0 || index >= value.attachmentCount() {
		return actorPlan{}, errors.New("route Attachment index is outside its bound")
	}
	result := value
	result.Attachments, result.AttachmentPlans = 0, nil
	if index > 0 {
		result.AcknowledgementSocket, result.AcknowledgementKey = "", ""
	}
	if len(value.AttachmentPlans) == 0 {
		return result, result.validateSingleRoleLocal()
	}
	override := value.AttachmentPlans[index]
	if override.Seed != "" {
		result.Seed = override.Seed
	}
	if override.ExcludedIdentities != nil {
		result.ExcludedIdentities = append([]string(nil), override.ExcludedIdentities...)
	}
	if override.ExcludedFamilies != nil {
		result.ExcludedFamilies = append([]string(nil), override.ExcludedFamilies...)
	}
	if override.ExcludedDomains != nil {
		result.ExcludedDomains = append([]string(nil), override.ExcludedDomains...)
	}
	for destination, source := range map[*string]string{
		&result.UpstreamPin: override.UpstreamPin, &result.NextNodeID: override.NextNodeID,
		&result.Next: override.Next, &result.NextPin: override.NextPin,
		&result.AcknowledgementSocket:     override.AcknowledgementSocket,
		&result.AcknowledgementKey:        override.AcknowledgementKey,
		&result.IntroductionSetupSocket:   override.IntroductionSetupSocket,
		&result.IntroductionSetupPublic:   override.IntroductionSetupPublic,
		&result.IntroductionServicePublic: override.IntroductionServicePublic,
		&result.IntroductionSetupPeer:     override.IntroductionSetupPeer,
		&result.IntroductionSetupNode:     override.IntroductionSetupNode,
		&result.Listen:                    override.Listen, &result.ServiceCertificate: override.ServiceCertificate,
		&result.ServiceKey: override.ServiceKey,
	} {
		if source != "" {
			*destination = source
		}
	}
	return result, result.validateSingleRoleLocal()
}
