package network

import (
	"time"
)

type PrivateMessagingStatus struct {
	Profile         string
	State           string
	SwitchReason    string
	RecoveryState   string
	ReducedFeatures []TransportFeature
	ErrorCategories []string
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
	ReducedFeatures         []TransportFeature
	ActiveFeatures          []TransportFeature
	AbuseState              string
	AbuseReason             string
	RateLimitedOperations   uint64
	BackpressuredOperations uint64
	OversizedMessages       uint64
	BannedProviders         int
	StoreEnabled            bool
	StoreState              string
	StoreMessages           int
	StoreCapacityMessages   int
	StoreCapacityBytes      int64
	StoreFileBytes          int64
	StoreUsageRatio         float64
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
		ReducedFeatures: append(append([]TransportFeature(nil), profile.ReducedFeatures...), privacyStatus.ReducedFeatures...),
		ActiveFeatures:  append([]TransportFeature(nil), profile.ActiveFeatures...), AbuseState: abuse.State, AbuseReason: abuse.Reason,
		RateLimitedOperations: abuse.RateLimitedOperations, BackpressuredOperations: abuse.BackpressuredOperations,
		OversizedMessages: abuse.OversizedMessages, BannedProviders: abuse.BannedProviders,
		StoreEnabled: abuse.StoreEnabled, StoreState: abuse.StoreState, StoreMessages: abuse.StoreMessages,
		StoreCapacityMessages: abuse.StoreCapacityMessages, StoreFileBytes: abuse.StoreFileBytes,
		StoreCapacityBytes: abuse.StoreCapacityBytes,
		StoreUsageRatio:    abuse.StoreUsageRatio, LastTransitionAt: lastTransitionAt,
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
