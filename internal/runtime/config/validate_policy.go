package config

import (
	"fmt"
	"slices"
	"time"
)

func validatePolicy(doc Document) error {
	policy := doc.Policy
	if policy.MaxWorkloads < 0 {
		return fmt.Errorf("policy.max_workloads cannot be negative")
	}
	if policy.MaxWorkloads > 0 && len(doc.Workloads.Initial) > policy.MaxWorkloads {
		return fmt.Errorf("policy.max_workloads is lower than the initial workload count")
	}
	if len(policy.AllowedPolicyRefs) > 0 && len(doc.Workloads.AllowedPolicyRefs) > 0 &&
		!sameStrings(policy.AllowedPolicyRefs, doc.Workloads.AllowedPolicyRefs) {
		return fmt.Errorf("policy.allowed_policy_refs conflicts with workloads.allowed_policy_refs")
	}
	localMax, err := policyDuration("policy.max_local_retention", policy.MaxLocalRetention)
	if err != nil {
		return err
	}
	relayMax, err := policyDuration("policy.max_relay_retention", policy.MaxRelayRetention)
	if err != nil {
		return err
	}
	if err := validateRetentionDefault("data.default_local_retention", doc.Data.DefaultLocalRetention, localMax); err != nil {
		return err
	}
	if err := validateRetentionDefault("data.default_relay_retention", doc.Data.DefaultRelayRetention, relayMax); err != nil {
		return err
	}
	if policy.DisableBlobPinning && policy.AllowPinRelayRetainedBlobs {
		return fmt.Errorf("policy cannot allow relay pinning while blob pinning is disabled")
	}
	if policy.DisablePeerBlobReserving && policy.AllowReservingRelayBlobs {
		return fmt.Errorf("policy cannot allow relay reservations while peer reserving is disabled")
	}
	return nil
}

func policyDuration(path, raw string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%s must be a non-negative duration", path)
	}
	return value, nil
}

func validateRetentionDefault(path, raw string, ceiling time.Duration) error {
	if raw == "" || ceiling == 0 {
		return nil
	}
	value, _ := time.ParseDuration(raw)
	if value > ceiling {
		return fmt.Errorf("%s exceeds its policy ceiling", path)
	}
	return nil
}

func sameStrings(left, right []string) bool {
	leftCopy, rightCopy := append([]string(nil), left...), append([]string(nil), right...)
	slices.Sort(leftCopy)
	slices.Sort(rightCopy)
	return slices.Equal(leftCopy, rightCopy)
}
