package provisioning

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// RunPreparePlan codifies the platform-led node-role provisioning recipe: it
// writes the role's exact runtime invocation declaration from operator inputs.
// It is pure configuration preparation (ADR-0125 decision 6 keeps validation with
// the owner). It performs no State acceptance, no hosting initialization and no
// root creation; those stay with network accept-*, hosting initialize and the
// runtime. Every refusal precedes the single output write.
func RunPreparePlan(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) == 0 {
		return 2
	}
	role := args[0]
	flags, err := preparePlanFlags(role, args[1:])
	if err != nil {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	if err := PrepareRuntimePlan(ctx, role, flags); err != nil {
		switch {
		case errors.Is(err, ErrRuntimePlanWrite):
			return 1
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return 130
		default:
			return 2
		}
	}
	receipt := struct {
		Operation  string `json:"operation"`
		Role       string `json:"role"`
		Outcome    string `json:"outcome"`
		Output     string `json:"output"`
		Validation string `json:"validation"`
	}{"node.prepare-plan", role, "prepared", flags["output"], "public-grammar-only"}
	if json.NewEncoder(out).Encode(receipt) != nil {
		return 2
	}
	return 0
}

// preparePlanFlags parses the exact closed flag set for one role. Every allowed
// flag is required; unknown, duplicate, foreign-role and valueless flags refuse.
func preparePlanFlags(role string, args []string) (map[string]string, error) {
	bad := errors.New("invalid prepare-plan arguments")
	allowed := map[string]bool{
		"network": true, "node-id": true, "participation-root": true, "spend-root": true, "hosting-root": true,
		"certificate": true, "private-key": true, "work": true, "termination": true, "retirement-wait-ms": true, "output": true,
	}
	switch role {
	case "forwarding", "join":
	case "descriptor":
		allowed["descriptor-root"] = true
	case "introduction":
		allowed["introduction-root"] = true
	case "issuer":
		allowed["issuance"] = true
		allowed["bootstrap-work"] = true
	default:
		return nil, bad
	}
	flags := map[string]string{}
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) || !strings.HasPrefix(args[i], "--") {
			return nil, bad
		}
		key := strings.TrimPrefix(args[i], "--")
		if !allowed[key] {
			return nil, bad
		}
		if _, duplicate := flags[key]; duplicate {
			return nil, bad
		}
		flags[key] = args[i+1]
	}
	for key := range allowed {
		if _, ok := flags[key]; !ok {
			return nil, bad
		}
	}
	return flags, nil
}
