package nameauthority

import (
	"errors"
	"sort"
	"strings"
	"time"
)

// RecoveryPolicy expresses who can execute recovery and how.
type RecoveryPolicy struct {
	Version     uint64
	Authorities []string
	Threshold   int
	Delay       time.Duration
}

// PolicyState stores active and pending recovery policy.
type PolicyState struct {
	Active           *RecoveryPolicy
	Pending          *RecoveryPolicy
	PendingActivated int64
}

// Config carries Stage 6 timing defaults for authority checks.
type Config struct {
	DefaultLeaseDuration time.Duration
	DefaultGraceDuration time.Duration
	DefaultPolicyDelay   time.Duration
}

func canonicalAuthorities(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, errors.New("recovery authorities list is empty")
	}
	seen := make(map[string]struct{}, len(raw))
	canonical := make([]string, 0, len(raw))
	for _, value := range raw {
		id := strings.TrimSpace(value)
		if id == "" {
			return nil, errors.New("recovery authority identifier is empty")
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		canonical = append(canonical, id)
	}
	sort.Strings(canonical)
	return canonical, nil
}

func normalizePolicy(policy RecoveryPolicy) (RecoveryPolicy, error) {
	ids, err := canonicalAuthorities(policy.Authorities)
	if err != nil {
		return RecoveryPolicy{}, err
	}
	normalized := RecoveryPolicy{Version: policy.Version, Authorities: ids, Threshold: policy.Threshold, Delay: policy.Delay}
	if normalized.Threshold < 1 {
		return RecoveryPolicy{}, errors.New("recovery threshold must be at least one")
	}
	if normalized.Threshold > len(normalized.Authorities) {
		return RecoveryPolicy{}, errors.New("recovery threshold exceeds authority count")
	}
	if normalized.Delay <= 0 {
		return RecoveryPolicy{}, errors.New("recovery delay must be positive")
	}
	if normalized.Version == 0 {
		normalized.Version = 1
	}
	return normalized, nil
}

func copyPolicy(policy *RecoveryPolicy) *RecoveryPolicy {
	if policy == nil {
		return nil
	}
	clone := *policy
	clone.Authorities = append([]string(nil), policy.Authorities...)
	return &clone
}

func copyPolicyState(state *PolicyState) *PolicyState {
	if state == nil {
		return &PolicyState{}
	}
	clone := *state
	clone.Active = copyPolicy(state.Active)
	clone.Pending = copyPolicy(state.Pending)
	return &clone
}

// ActivatePolicy applies a pending policy when its change window elapsed.
func ActivatePolicy(state *PolicyState, now int64) *PolicyState {
	if state == nil {
		return nil
	}
	clone := copyPolicyState(state)
	if clone.Pending == nil || clone.PendingActivated == 0 || now < clone.PendingActivated {
		return clone
	}
	clone.Active = clone.Pending
	clone.Pending = nil
	clone.PendingActivated = 0
	return clone
}

// CanWitness validates witness cardinality and identity set membership.
func CanWitness(policy *RecoveryPolicy, actor string, witnesses []string) bool {
	if policy == nil || actor == "" {
		return false
	}
	allowed := make(map[string]struct{}, len(policy.Authorities))
	for _, value := range policy.Authorities {
		allowed[value] = struct{}{}
	}
	checked := make(map[string]struct{}, len(witnesses)+1)
	count := 0
	add := func(id string) {
		if _, seen := checked[id]; seen {
			return
		}
		checked[id] = struct{}{}
		if _, ok := allowed[id]; ok {
			count++
		}
	}
	add(actor)
	for _, value := range witnesses {
		add(strings.TrimSpace(value))
	}
	return count >= policy.Threshold
}

func activeRecoveryPolicy(state *PolicyState) (*RecoveryPolicy, bool) {
	if state == nil || state.Active == nil {
		return nil, false
	}
	return copyPolicy(state.Active), true
}
