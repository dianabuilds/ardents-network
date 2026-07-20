package process

import apppolicy "ardents/internal/policy"

func NewPolicyService(cfg PolicyConfig) *apppolicy.Service {
	return apppolicy.New(runtimePolicyConfig(cfg))
}
