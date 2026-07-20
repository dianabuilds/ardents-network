package readiness

import "fmt"

type StartupVariant string

const (
	StartupVariantTCPOnly StartupVariant = "tcp_only"
	StartupVariantTCPWSS  StartupVariant = "tcp_wss"
	StartupVariantTCPQUIC StartupVariant = "tcp_quic"
)

type Definition struct {
	Profile            Profile
	Implemented        bool
	StartupVariant     StartupVariant
	ActiveFamilies     []Family
	SuppressedFamilies []Family
}

type RuntimeShape struct {
	ActiveFamilies     []Family
	SuppressedFamilies []Family
}

func NormalizeProfile(profile Profile) Profile {
	if profile == "" {
		return ProfileTCPOnly
	}
	return profile
}

func (d Definition) RuntimeShape() RuntimeShape {
	return RuntimeShape{
		ActiveFamilies:     cloneFamilies(d.ActiveFamilies),
		SuppressedFamilies: cloneFamilies(d.SuppressedFamilies),
	}
}

func LookupProfile(profile Profile) Definition {
	switch NormalizeProfile(profile) {
	case ProfileTCPOnly:
		return Definition{
			Profile:            ProfileTCPOnly,
			Implemented:        true,
			StartupVariant:     StartupVariantTCPOnly,
			ActiveFamilies:     []Family{FamilyTCP},
			SuppressedFamilies: []Family{FamilyQUIC, FamilyWSS, FamilyWebTransport, FamilyWebRTC},
		}
	case ProfileTCPWSS:
		return Definition{
			Profile:            ProfileTCPWSS,
			Implemented:        true,
			StartupVariant:     StartupVariantTCPWSS,
			ActiveFamilies:     []Family{FamilyTCP, FamilyWSS},
			SuppressedFamilies: []Family{FamilyQUIC, FamilyWebTransport, FamilyWebRTC},
		}
	case ProfileTCPQUIC:
		return Definition{
			Profile:            ProfileTCPQUIC,
			Implemented:        false,
			StartupVariant:     StartupVariantTCPQUIC,
			SuppressedFamilies: []Family{FamilyWSS, FamilyWebTransport, FamilyWebRTC},
		}
	default:
		return Definition{}
	}
}

func ResolveProfile(profile Profile) (Definition, error) {
	definition := LookupProfile(profile)
	if definition.Profile == "" {
		return Definition{}, fmt.Errorf("transport profile %q is unknown", profile)
	}
	if !definition.Implemented {
		return Definition{}, fmt.Errorf("transport profile %q is not implemented", definition.Profile)
	}
	return definition, nil
}

func RuntimeShapeForProfile(profile Profile) (RuntimeShape, error) {
	definition, err := ResolveProfile(profile)
	if err != nil {
		return RuntimeShape{}, err
	}
	return definition.RuntimeShape(), nil
}

func cloneFamilies(in []Family) []Family {
	if len(in) == 0 {
		return nil
	}
	return append([]Family(nil), in...)
}
