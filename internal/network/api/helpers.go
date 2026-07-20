package api

import (
	"time"

	networkreadiness "ardents/internal/network/readiness"
	networktransport "ardents/internal/network/transport"
)

func ValidateTransportConfig(cfg Config, now time.Time) error {
	return networktransport.ValidateConfig(cfg, now)
}

func New(cfg ...Config) *networktransport.Service {
	return networktransport.New(cfg...)
}

func NormalizeProfile(profile Profile) Profile {
	return networkreadiness.NormalizeProfile(profile)
}

func LookupProfile(profile Profile) networkreadiness.Definition {
	return networkreadiness.LookupProfile(profile)
}

func ResolveProfile(profile Profile) (networkreadiness.Definition, error) {
	return networkreadiness.ResolveProfile(profile)
}

func RuntimeShapeForProfile(profile Profile) (networkreadiness.RuntimeShape, error) {
	return networkreadiness.RuntimeShapeForProfile(profile)
}
