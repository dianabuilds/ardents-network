package nameresolver

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/namelease"
)

// ResolverQuery selects either name lookup or endpoint-observability role.
type ResolverQuery struct {
	Role     string
	Name     string
	Target   string
	Observer string
}

// ResolverResult captures one role-scoped resolution outcome.
type ResolverResult struct {
	Role       string
	Name       string
	Target     string
	Generation uint64
	Revision   uint64
	State      string
	Warning    string
}

const (
	roleLookup   = "lookup"
	roleEndpoint = "endpoint"
	roleObserver = "observer"
)

// Resolve enforces role-scoped visibility. No call can receive both exact name and
// target-observed view in one query context.
func Resolve(current namelease.Record, now int64, query ResolverQuery) (ResolverResult, error) {
	empty := ResolverResult{}
	switch query.Role {
	case roleLookup:
		if query.Name == "" {
			return empty, errors.New("lookup role requires Service Name")
		}
		if query.Target != "" {
			return empty, errors.New("lookup role cannot include endpoint target")
		}
		if query.Name != current.Name {
			return empty, errors.New("lookup name mismatch")
		}
		canResolve, warning := namelease.CanResolve(current, now)
		if !canResolve {
			return empty, errors.New(warning)
		}
		return ResolverResult{
			Role:       roleLookup,
			Name:       current.Name,
			Target:     current.Target,
			Generation: current.Generation,
			Revision:   current.Revision,
			State:      current.State,
			Warning:    warning,
		}, nil
	case roleEndpoint:
		if query.Target == "" {
			return empty, errors.New("endpoint role requires target context")
		}
		if query.Name != "" {
			return empty, errors.New("endpoint role must not include exact name")
		}
		if query.Target != current.Target {
			return empty, errors.New("endpoint target mismatch")
		}
		canResolve, warning := namelease.CanResolve(current, now)
		if !canResolve {
			return empty, errors.New(warning)
		}
		return ResolverResult{
			Role:       roleEndpoint,
			Name:       "",
			Target:     current.Target,
			Generation: current.Generation,
			Revision:   current.Revision,
			State:      current.State,
			Warning:    warning,
		}, nil
	case roleObserver:
		if query.Name != "" || query.Target != "" {
			return empty, errors.New("observer role must not request lookup fields")
		}
		return ResolverResult{
			Role:    roleObserver,
			State:   current.State,
			Name:    "",
			Target:  "",
			Warning: "observer-only view",
		}, nil
	default:
		return empty, errors.New("unknown resolver role")
	}
}

// Must be called from role-local callers that still need a deterministic timeout wall.
func IsActiveWindow(now int64, record namelease.Record) bool {
	canResolve, _ := namelease.CanResolve(record, now)
	return canResolve
}

// IsFresh ensures records do not contain future timestamps for this now.
func IsFresh(now int64, record namelease.Record) bool {
	if record.State == "" {
		return false
	}
	switch record.State {
	case "active":
		return record.LeaseExpiresAt >= now
	case "grace":
		return record.GraceExpiresAt >= now
	case "released", "recovery-pending", "conflict":
		return true
	default:
		return false
	}
}
