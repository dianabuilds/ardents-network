package api

import (
	networkmessaging "ardents/internal/network/messaging"
	networkreadiness "ardents/internal/network/readiness"
	networkroute "ardents/internal/network/route"
	networktransport "ardents/internal/network/transport"
)

type Config = networktransport.Config
type Limits = networktransport.Limits
type AbuseSnapshot = networktransport.AbuseSnapshot

type Envelope = networkmessaging.Envelope
type Candidate = networkroute.Candidate
type DiscoveryPublishError = networkmessaging.DiscoveryPublishError

type BootstrapStatus = networkreadiness.BootstrapStatus
type BootstrapDialReport = networkreadiness.BootstrapDialReport
type HealthSignals = networkreadiness.HealthSignals

type Profile = networkreadiness.Profile
type NodeProfile = networkreadiness.NodeProfile
type NodeProfileDefinition = networkreadiness.NodeProfileDefinition
type Mode = networkreadiness.Mode
type Family = networkreadiness.Family
type HealthState = networkreadiness.HealthState
type SwitchReason = networkreadiness.SwitchReason
type RecoveryState = networkreadiness.RecoveryState
type Snapshot = networkreadiness.Snapshot
type ReachabilityMode = networkreadiness.ReachabilityMode
type ReachabilitySnapshot = networkreadiness.ReachabilitySnapshot

type TransportProfile = networkreadiness.Profile
type TransportMode = networkreadiness.Mode
type TransportFamily = networkreadiness.Family
type TransportHealthState = networkreadiness.HealthState
type TransportSwitchReason = networkreadiness.SwitchReason
type TransportRecoveryState = networkreadiness.RecoveryState
type TransportSnapshot = networkreadiness.Snapshot

const DefaultPubsubTopic = networkmessaging.DefaultPubsubTopic
const BindAddressEnv = networktransport.BindAddressEnv

const ProfileTCPOnly = networkreadiness.ProfileTCPOnly
const ProfileTCPQUIC = networkreadiness.ProfileTCPQUIC
const ProfileTCPWSS = networkreadiness.ProfileTCPWSS

const NodeProfileServiceNode = networkreadiness.NodeProfileServiceNode
const NodeProfileConstrainedClient = networkreadiness.NodeProfileConstrainedClient
const NodeProfileLocalDevelopment = networkreadiness.NodeProfileLocalDevelopment
const NodeProfileRestrictedDefense = networkreadiness.NodeProfileRestrictedDefense

const ModeSteady = networkreadiness.ModeSteady
const ModeRestrictedDefense = networkreadiness.ModeRestrictedDefense

const ReachabilityLocalOnly = networkreadiness.ReachabilityLocalOnly
const ReachabilityPrivateLAN = networkreadiness.ReachabilityPrivateLAN
const ReachabilityPublicDirect = networkreadiness.ReachabilityPublicDirect
const ReachabilityOutboundOnly = networkreadiness.ReachabilityOutboundOnly
