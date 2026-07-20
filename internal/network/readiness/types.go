package readiness

type Profile string

const (
	ProfileTCPOnly Profile = "tcp_only"
	ProfileTCPQUIC Profile = "tcp_quic"
	ProfileTCPWSS  Profile = "tcp_wss"
)

type Mode string

const (
	ModeSteady            Mode = "steady"
	ModeRestrictedDefense Mode = "restricted_defense"
)

type Family string

const (
	FamilyTCP          Family = "tcp"
	FamilyQUIC         Family = "quic"
	FamilyWSS          Family = "wss"
	FamilyWebTransport Family = "webtransport"
	FamilyWebRTC       Family = "webrtc"
)

type HealthState string

const (
	HealthStateUnknown  HealthState = "unknown"
	HealthStateReady    HealthState = "ready"
	HealthStateDegraded HealthState = "degraded"
	HealthStateFailed   HealthState = "failed"
	HealthStateStopped  HealthState = "stopped"
	HealthStateStarting HealthState = "starting"
)

type SwitchReason string

const (
	SwitchReasonStartupDefault    SwitchReason = "startup_default"
	SwitchReasonBootstrapDegraded SwitchReason = "bootstrap_degraded"
	SwitchReasonHealthDegraded    SwitchReason = "health_degraded"
	SwitchReasonRecoveryReady     SwitchReason = "recovery_ready"
	SwitchReasonStartupFailed     SwitchReason = "startup_failed"
	SwitchReasonStopped           SwitchReason = "stopped"
	SwitchReasonNotStarted        SwitchReason = "not_started"
)

type RecoveryState string

const (
	RecoveryStateSteady          RecoveryState = "steady"
	RecoveryStateRecoveryPending RecoveryState = "recovery_pending"
	RecoveryStateCooldownActive  RecoveryState = "cooldown_active"
	RecoveryStateBlocked         RecoveryState = "blocked"
)

type Snapshot struct {
	NodeProfile         NodeProfile
	Profile             Profile
	Mode                Mode
	Health              HealthState
	ActiveFamilies      []Family
	SuppressedFamilies  []Family
	SwitchReason        SwitchReason
	SwitchAutomatic     bool
	ReducedCapabilities []string
	ActiveCapabilities  []string
	RecoveryState       RecoveryState
}
