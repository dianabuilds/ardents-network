package policy

import "ardents/internal/policy/policyset"

type Config = policyset.Config

func normalizeConfig(cfg Config) Config {
	return policyset.Normalize(cfg)
}
