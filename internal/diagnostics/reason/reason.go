package reason

type Reason struct {
	Code                   string `json:"code"`
	Domain                 string `json:"domain"`
	Summary                string `json:"summary"`
	Detail                 string `json:"detail,omitempty"`
	Impact                 string `json:"impact,omitempty"`
	Recovery               string `json:"recovery,omitempty"`
	OperatorActionRequired bool   `json:"operator_action_required"`
	Resource               string `json:"resource,omitempty"`
}

func Clone(in *Reason) *Reason {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
