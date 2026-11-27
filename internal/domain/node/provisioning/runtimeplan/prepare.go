package runtimeplan

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	nodeplan "github.com/dianabuilds/ardents-network/internal/domain/node/plan"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/platform/processguard"
)

var errRuntimePlan = errors.New("invalid runtime plan input")

// errRuntimePlanWrite is a failure to finish writing an exclusively created plan.
var errRuntimePlanWrite = errors.New("runtime plan write failed")

// prepare creates one detached runtime declaration. It uses the
// selected role's codec and creates no semantic root or signed authority.
func prepare(ctx context.Context, role string, flags map[string]string) error {
	if ctx == nil {
		return errRuntimePlan
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, name := range []string{"network", "participation-root", "spend-root", "hosting-root", "certificate", "private-key", "output"} {
		if !pathcheck.IsCleanAbsolute(flags[name]) {
			return errRuntimePlan
		}
	}
	networkRaw, err := artifacts.ReadBoundedFile(flags["network"], 16<<10)
	if err != nil {
		return errRuntimePlan
	}
	network, err := nodeplan.DecodeNodeNetwork(networkRaw)
	if err != nil {
		return errRuntimePlan
	}
	id, ok := canonicaldigest.Decode(flags["node-id"])
	if !ok {
		return errRuntimePlan
	}
	work, err := preparePlanTraffic(flags["work"])
	if err != nil {
		return errRuntimePlan
	}
	termination, err := preparePlanTraffic(flags["termination"])
	if err != nil {
		return errRuntimePlan
	}
	wait, err := parsePlanNumber(flags["retirement-wait-ms"], 32)
	if err != nil || wait == 0 || wait > 5000 {
		return errRuntimePlan
	}
	profile, roots, issuanceRaw, bootstrap, err := preparePlanRole(role, flags)
	if err != nil {
		return errRuntimePlan
	}
	authority := (*state.AuthorityDeclaration)(&network)
	if !authority.ValidShape() || !network.IndependentRoots(roots...) {
		return errRuntimePlan
	}
	plan := map[string]any{
		"network": network, "node_id": id, "process_profile": profile,
		"participation_root": flags["participation-root"], "spend_root": flags["spend-root"], "hosting_root": flags["hosting-root"],
		"certificate": flags["certificate"], "private_key": flags["private-key"],
		"work": work, "termination": termination, "retirement_wait_ms": uint32(wait),
	}
	switch role {
	case "descriptor":
		plan["descriptor_root"] = flags["descriptor-root"]
	case "introduction":
		plan["introduction_root"] = flags["introduction-root"]
	case "issuer":
		plan["issuance"] = json.RawMessage(issuanceRaw)
		plan["bootstrap_work"] = bootstrap
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return errRuntimePlan
	}
	if err := preparePlanGrammar(role, raw); err != nil {
		return errRuntimePlan
	}
	f, err := os.OpenFile(flags["output"], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errRuntimePlan
	}
	_, writeErr := f.Write(append(raw, '\n'))
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return errors.Join(errRuntimePlanWrite, err)
	}
	return ctx.Err()
}

// preparePlanRole selects the role's process profile and writable-root list and
// acquires the Issuer initialize-results sub-plan. The per-role policy stays here
// in the declaration producer.
func preparePlanRole(role string, flags map[string]string) (profile string, roots []string, issuanceRaw []byte, bootstrap hosting.Traffic, err error) {
	bad := errors.New("invalid prepare-plan role input")
	roots = []string{flags["participation-root"], flags["spend-root"], flags["hosting-root"]}
	switch role {
	case "forwarding":
		profile = processguard.ForwardingProfile
	case "join":
		profile = processguard.JoinProfile
	case "descriptor":
		if !pathcheck.IsCleanAbsolute(flags["descriptor-root"]) {
			return "", nil, nil, bootstrap, bad
		}
		profile = processguard.DescriptorProfile
		roots = append(roots, flags["descriptor-root"])
	case "introduction":
		if !pathcheck.IsCleanAbsolute(flags["introduction-root"]) {
			return "", nil, nil, bootstrap, bad
		}
		profile = processguard.IntroductionProfile
		roots = append(roots, flags["introduction-root"])
	case "issuer":
		if !pathcheck.IsCleanAbsolute(flags["issuance"]) {
			return "", nil, nil, bootstrap, bad
		}
		profile = processguard.IssuerProfile
		issuanceRaw, err = artifacts.ReadBoundedFile(flags["issuance"], 16<<10)
		if err != nil {
			return "", nil, nil, bootstrap, bad
		}
		prepared, e := issuance.DecodeResultSetupDeclaration(issuanceRaw)
		if e != nil {
			return "", nil, nil, bootstrap, bad
		}
		bootstrap, err = preparePlanTraffic(flags["bootstrap-work"])
		if err != nil {
			return "", nil, nil, bootstrap, bad
		}
		roots = append(roots, prepared.AdmissionRoot, prepared.KeyRoot, prepared.ResultRoot)
	default:
		return "", nil, nil, bootstrap, bad
	}
	return profile, roots, issuanceRaw, bootstrap, nil
}

// preparePlanGrammar confirms the produced bytes decode through the same closed
// Node codec the runtime consumes. The result supplies declaration facts only.
func preparePlanGrammar(role string, raw []byte) error {
	switch role {
	case "forwarding":
		_, err := nodeplan.DecodeForwardingPlan(raw)
		return err
	case "join":
		_, err := nodeplan.DecodeJoinPlan(raw)
		return err
	case "descriptor":
		_, err := nodeplan.DecodeDescriptorPlan(raw)
		return err
	case "introduction":
		_, err := nodeplan.DecodeIntroductionPlan(raw)
		return err
	case "issuer":
		_, err := nodeplan.DecodeIssuerPlan(raw)
		return err
	}
	return errors.New("invalid prepare-plan role")
}

// preparePlanTraffic parses an upper Hosting reservation as canonical "tx:rx"
// byte counts. It describes a reservation, not a measurement or an admission.
func preparePlanTraffic(value string) (hosting.Traffic, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return hosting.Traffic{}, errors.New("invalid traffic reservation")
	}
	tx, err := parsePlanNumber(parts[0], 64)
	if err != nil {
		return hosting.Traffic{}, err
	}
	rx, err := parsePlanNumber(parts[1], 64)
	if err != nil {
		return hosting.Traffic{}, err
	}
	return hosting.Traffic{Tx: tx, Rx: rx}, nil
}

func parsePlanNumber(value string, bits int) (uint64, error) {
	number, err := strconv.ParseUint(value, 10, bits)
	if err != nil || strconv.FormatUint(number, 10) != value {
		return 0, errRuntimePlan
	}
	return number, nil
}
