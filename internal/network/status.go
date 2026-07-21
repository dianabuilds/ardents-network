package network

import (
	"time"
)

type PrivateMessagingStatus struct {
	Profile             string
	State               string
	SwitchReason        string
	RecoveryState       string
	ReducedCapabilities []string
	ErrorCategories     []string
}

type StatusSnapshot struct {
	State                   string
	Reason                  string
	Joined                  bool
	Reachable               bool
	ReachabilityMode        string
	ReachabilityState       string
	ReachabilityReason      string
	ReachabilityObservedAt  time.Time
	ActiveProfile           string
	ActiveMode              string
	ReducedCapabilities     []string
	ActiveCapabilities      []string
	AbuseState              string
	AbuseReason             string
	RateLimitedOperations   uint64
	BackpressuredOperations uint64
	OversizedMessages       uint64
	BannedProviders         int
	LastTransitionAt        time.Time
	PrivacyProfile          string
	PrivacyState            string
	PrivacySwitchReason     string
	PrivacyRecoveryState    string
	PrivacyErrors           []string
	NodeProfile             string
}

func ProjectStatus(nodeProfile NodeProfile, state, reason string, joined bool, profile Snapshot, reachability ReachabilitySnapshot, abuse AbuseSnapshot, lastTransitionAt time.Time, privacyStatus PrivateMessagingStatus) StatusSnapshot {
	return StatusSnapshot{
		NodeProfile: string(nodeProfile), State: state, Reason: reason, Joined: joined,
		Reachable: reachability.Reachable, ReachabilityMode: string(reachability.Mode), ReachabilityState: reachability.State,
		ReachabilityReason: reachability.Reason, ReachabilityObservedAt: reachability.ObservedAt,
		ActiveProfile: string(profile.Profile), ActiveMode: string(profile.Mode),
		ReducedCapabilities: append(append([]string(nil), profile.ReducedCapabilities...), privacyStatus.ReducedCapabilities...),
		ActiveCapabilities:  append([]string(nil), profile.ActiveCapabilities...), AbuseState: abuse.State, AbuseReason: abuse.Reason,
		RateLimitedOperations: abuse.RateLimitedOperations, BackpressuredOperations: abuse.BackpressuredOperations,
		OversizedMessages: abuse.OversizedMessages, BannedProviders: abuse.BannedProviders, LastTransitionAt: lastTransitionAt,
		PrivacyProfile: privacyStatus.Profile, PrivacyState: privacyStatus.State, PrivacySwitchReason: privacyStatus.SwitchReason,
		PrivacyRecoveryState: privacyStatus.RecoveryState, PrivacyErrors: append([]string(nil), privacyStatus.ErrorCategories...),
	}
}

func PeerReachability(candidates []Candidate) (string, string) {
	if len(candidates) == 0 {
		return "unreachable", "peer has no advertised endpoints"
	}
	for _, candidate := range candidates {
		if candidate.Usable {
			return "reachable", ""
		}
	}
	return "limited", "peer endpoints are not currently usable"
}
