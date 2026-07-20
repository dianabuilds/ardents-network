package readiness

import "time"

type SelectionPolicy struct {
	DegradedThreshold int
	RecoveryThreshold int
	Cooldown          time.Duration
}

type ModeDecision struct {
	Mode          Mode
	Changed       bool
	Reason        SwitchReason
	Automatic     bool
	CooldownUntil time.Time
	RecoveryState RecoveryState
}

type ModeController struct {
	policy              SelectionPolicy
	consecutiveDegraded int
	consecutiveReady    int
	cooldownUntil       time.Time
}

func DefaultSelectionPolicy() SelectionPolicy {
	return SelectionPolicy{DegradedThreshold: 3, RecoveryThreshold: 2, Cooldown: 30 * time.Second}
}

func NewModeController(policy SelectionPolicy) *ModeController {
	if policy.DegradedThreshold < 1 {
		policy.DegradedThreshold = DefaultSelectionPolicy().DegradedThreshold
	}
	if policy.RecoveryThreshold < 1 {
		policy.RecoveryThreshold = DefaultSelectionPolicy().RecoveryThreshold
	}
	if policy.Cooldown <= 0 {
		policy.Cooldown = DefaultSelectionPolicy().Cooldown
	}
	return &ModeController{policy: policy}
}

func (c *ModeController) Evaluate(now time.Time, current Mode, health HealthState) ModeDecision {
	if current == "" {
		current = ModeSteady
	}
	if current == ModeRestrictedDefense {
		return c.evaluateDefense(now, health)
	}
	return c.evaluateSteady(now, health)
}

func (c *ModeController) evaluateSteady(now time.Time, health HealthState) ModeDecision {
	if now.Before(c.cooldownUntil) {
		c.consecutiveReady = 0
		return ModeDecision{Mode: ModeSteady, CooldownUntil: c.cooldownUntil, RecoveryState: RecoveryStateSteady}
	}
	switch health {
	case HealthStateDegraded, HealthStateFailed:
		c.consecutiveDegraded++
		c.consecutiveReady = 0
	default:
		c.consecutiveDegraded = 0
		c.consecutiveReady++
	}
	if c.consecutiveDegraded < c.policy.DegradedThreshold {
		recoveryState := RecoveryStateSteady
		if health == HealthStateDegraded || health == HealthStateFailed {
			recoveryState = RecoveryStateRecoveryPending
		}
		return ModeDecision{Mode: ModeSteady, CooldownUntil: c.cooldownUntil, RecoveryState: recoveryState}
	}
	c.cooldownUntil = now.Add(c.policy.Cooldown)
	c.consecutiveDegraded = 0
	c.consecutiveReady = 0
	return ModeDecision{
		Mode:          ModeRestrictedDefense,
		Changed:       true,
		Reason:        SwitchReasonHealthDegraded,
		Automatic:     true,
		CooldownUntil: c.cooldownUntil,
		RecoveryState: RecoveryStateCooldownActive,
	}
}

func (c *ModeController) evaluateDefense(now time.Time, health HealthState) ModeDecision {
	if now.Before(c.cooldownUntil) {
		c.consecutiveReady = 0
		return ModeDecision{Mode: ModeRestrictedDefense, CooldownUntil: c.cooldownUntil, RecoveryState: RecoveryStateRecoveryPending}
	}
	switch health {
	case HealthStateReady:
		c.consecutiveReady++
		c.consecutiveDegraded = 0
	default:
		c.consecutiveReady = 0
		c.consecutiveDegraded++
	}
	if c.consecutiveReady < c.policy.RecoveryThreshold {
		return ModeDecision{Mode: ModeRestrictedDefense, CooldownUntil: c.cooldownUntil, RecoveryState: RecoveryStateRecoveryPending}
	}
	c.cooldownUntil = now.Add(c.policy.Cooldown)
	c.consecutiveReady = 0
	c.consecutiveDegraded = 0
	return ModeDecision{
		Mode:          ModeSteady,
		Changed:       true,
		Reason:        SwitchReasonRecoveryReady,
		Automatic:     true,
		CooldownUntil: c.cooldownUntil,
		RecoveryState: RecoveryStateCooldownActive,
	}
}
