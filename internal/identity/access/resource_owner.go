package access

import identityprincipal "ardents/internal/identity/principal"

// ResourceOwner is the closed owner representation used by identity/access.
// The zero value means that the resource kind is not owner-scoped; the only
// non-zero variant is a canonical Principal.
type ResourceOwner struct {
	principal identityprincipal.ID
}

func PrincipalOwner(principal identityprincipal.ID) (ResourceOwner, error) {
	if principal.String() == "" {
		return ResourceOwner{}, ErrInvalidArgument
	}
	return ResourceOwner{principal: principal}, nil
}

func ParseResourceOwner(value string) (ResourceOwner, error) {
	if value == "" {
		return ResourceOwner{}, nil
	}
	principal, err := identityprincipal.Parse(value)
	if err != nil {
		return ResourceOwner{}, ErrInvalidArgument
	}
	return PrincipalOwner(principal)
}

func (owner ResourceOwner) String() string { return owner.principal.String() }
func (owner ResourceOwner) IsNone() bool   { return owner.principal.String() == "" }
func (owner ResourceOwner) Equal(other ResourceOwner) bool {
	return owner.principal.Equal(other.principal)
}
func (owner ResourceOwner) Principal() (identityprincipal.ID, bool) {
	return owner.principal, !owner.IsNone()
}
