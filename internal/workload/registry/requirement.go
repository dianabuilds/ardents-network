package registry

import (
	"encoding/json"
	"fmt"
)

const MaxWorkloadRequirementBytes = 64

// WorkloadRequirement names one execution or resource property required by a
// Node-scoped workload. It is not an Access Grant or a channel capability.
type WorkloadRequirement string

func ParseWorkloadRequirement(value string) (WorkloadRequirement, error) {
	if !validWorkloadRequirement(value) {
		return "", fmt.Errorf("invalid workload requirement")
	}
	return WorkloadRequirement(value), nil
}

func (r WorkloadRequirement) String() string { return string(r) }

func (r WorkloadRequirement) Valid() bool {
	return validWorkloadRequirement(string(r))
}

func ValidateWorkloadRequirements(values []WorkloadRequirement) error {
	seen := make(map[WorkloadRequirement]struct{}, len(values))
	for _, value := range values {
		if !value.Valid() {
			return fmt.Errorf("invalid workload requirement")
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("duplicate workload requirement")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func (r WorkloadRequirement) MarshalJSON() ([]byte, error) {
	if !r.Valid() {
		return nil, fmt.Errorf("invalid workload requirement")
	}
	return json.Marshal(string(r))
}

func (r *WorkloadRequirement) UnmarshalJSON(raw []byte) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("invalid workload requirement")
	}
	parsed, err := ParseWorkloadRequirement(value)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}

func validWorkloadRequirement(value string) bool {
	if len(value) == 0 || len(value) > MaxWorkloadRequirementBytes {
		return false
	}
	separator := false
	for index := 0; index < len(value); index++ {
		ch := value[index]
		if ch >= 'a' && ch <= 'z' || index > 0 && ch >= '0' && ch <= '9' {
			separator = false
			continue
		}
		if index > 0 && index < len(value)-1 && !separator && (ch == '.' || ch == '-' || ch == '_') {
			separator = true
			continue
		}
		return false
	}
	return !separator
}
