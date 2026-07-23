package identity

import "errors"

var ErrInvalidResourceOwner = errors.New("identity resource owner is invalid")

// ResourceOwner is either absent or a canonical Principal ID. The private
// representation prevents Node, Workload, Service, and arbitrary strings from
// becoming security owners in SDK calls.
type ResourceOwner struct {
	principal string
}

func PrincipalOwner(principal string) (ResourceOwner, error) {
	if !validPrincipalID(principal) {
		return ResourceOwner{}, ErrInvalidResourceOwner
	}
	return ResourceOwner{principal: principal}, nil
}

func ParseResourceOwner(value string) (ResourceOwner, error) {
	if value == "" {
		return ResourceOwner{}, nil
	}
	return PrincipalOwner(value)
}

func (owner ResourceOwner) String() string { return owner.principal }
func (owner ResourceOwner) IsNone() bool   { return owner.principal == "" }
func (owner ResourceOwner) Equal(other ResourceOwner) bool {
	return owner.principal == other.principal
}
