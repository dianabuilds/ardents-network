package network

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestModeControllerSelectsRestrictedDefenseAfterThreshold(t *testing.T) {
	controller := NewModeController(SelectionPolicy{
		DegradedThreshold: 2,
		RecoveryThreshold: 2,
		Cooldown:          time.Second,
	})
	now := time.Now().UTC()

	first := controller.Evaluate(now, ModeSteady, HealthStateDegraded)
	require.False(t, first.Changed)

	second := controller.Evaluate(now.Add(10*time.Millisecond), ModeSteady, HealthStateDegraded)
	require.True(t, second.Changed)
	require.Equal(t, ModeRestrictedDefense, second.Mode)
	require.Equal(t, SwitchReasonHealthDegraded, second.Reason)
}

func TestModeControllerHonorsCooldownBeforeRecovery(t *testing.T) {
	controller := NewModeController(SelectionPolicy{
		DegradedThreshold: 1,
		RecoveryThreshold: 1,
		Cooldown:          time.Minute,
	})
	now := time.Now().UTC()

	degraded := controller.Evaluate(now, ModeSteady, HealthStateFailed)
	require.True(t, degraded.Changed)

	recovery := controller.Evaluate(now.Add(time.Second), ModeRestrictedDefense, HealthStateReady)
	require.False(t, recovery.Changed)
	require.Equal(t, ModeRestrictedDefense, recovery.Mode)
}

func TestModeControllerRecoversToSteadyAfterRecoveryThreshold(t *testing.T) {
	controller := NewModeController(SelectionPolicy{
		DegradedThreshold: 1,
		RecoveryThreshold: 2,
		Cooldown:          time.Second,
	})
	now := time.Now().UTC()

	switchDecision := controller.Evaluate(now, ModeSteady, HealthStateDegraded)
	require.True(t, switchDecision.Changed)

	duringCooldown := controller.Evaluate(now.Add(500*time.Millisecond), ModeRestrictedDefense, HealthStateReady)
	require.False(t, duringCooldown.Changed)

	firstReady := controller.Evaluate(now.Add(2*time.Second), ModeRestrictedDefense, HealthStateReady)
	require.False(t, firstReady.Changed)

	secondReady := controller.Evaluate(now.Add(3*time.Second), ModeRestrictedDefense, HealthStateReady)
	require.True(t, secondReady.Changed)
	require.Equal(t, ModeSteady, secondReady.Mode)
	require.Equal(t, SwitchReasonRecoveryReady, secondReady.Reason)
}
