package provisioning

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

var errRuntimePlan = errors.New("invalid runtime plan input")

// ErrRuntimePlanWrite is a failure to finish writing an exclusively created plan.
var ErrRuntimePlanWrite = errors.New("runtime plan write failed")

// PrepareRuntimePlan creates one detached runtime declaration. It uses the
// selected role's codec and creates no semantic root or signed authority.
func PrepareRuntimePlan(ctx context.Context, role string, flags map[string]string) error {
	if ctx == nil {
		return errRuntimePlan
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, name := range []string{"network", "participation-root", "spend-root", "hosting-root", "certificate", "private-key", "output"} {
		if !pathgrammar.Strict(flags[name]) {
			return errRuntimePlan
		}
	}
	networkRaw, err := artifacts.ReadBoundedFile(flags["network"], 16<<10)
	if err != nil {
		return errRuntimePlan
	}
	network, err := node.DecodeNodeNetwork(networkRaw)
	if err != nil {
		return errRuntimePlan
	}
	nodeID, err := hex.DecodeString(flags["node-id"])
	if err != nil || len(nodeID) != 32 || hex.EncodeToString(nodeID) != flags["node-id"] {
		return errRuntimePlan
	}
	var id [32]byte
	copy(id[:], nodeID)
	work, err := preparePlanTraffic(flags["work"])
	if err != nil {
		return errRuntimePlan
	}
	termination, err := preparePlanTraffic(flags["termination"])
	if err != nil {
		return errRuntimePlan
	}
	wait, err := admissionNumber(flags["retirement-wait-ms"], 32)
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
		return errors.Join(ErrRuntimePlanWrite, err)
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
		profile = processresource.Profile
	case "join":
		profile = processresource.JoinProfile
	case "descriptor":
		if !pathgrammar.Strict(flags["descriptor-root"]) {
			return "", nil, nil, bootstrap, bad
		}
		profile = processresource.DescriptorProfile
		roots = append(roots, flags["descriptor-root"])
	case "introduction":
		if !pathgrammar.Strict(flags["introduction-root"]) {
			return "", nil, nil, bootstrap, bad
		}
		profile = processresource.IntroductionProfile
		roots = append(roots, flags["introduction-root"])
	case "issuer":
		if !pathgrammar.Strict(flags["issuance"]) {
			return "", nil, nil, bootstrap, bad
		}
		profile = processresource.IssuerProfile
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
// node codec the runtime consumes, so the written plan is runtime-admissible.
func preparePlanGrammar(role string, raw []byte) error {
	switch role {
	case "forwarding":
		_, err := node.DecodeForwardingPlan(raw)
		return err
	case "join":
		_, err := node.DecodeJoinPlan(raw)
		return err
	case "descriptor":
		_, err := node.DecodeDescriptorPlan(raw)
		return err
	case "introduction":
		_, err := node.DecodeIntroductionPlan(raw)
		return err
	case "issuer":
		_, err := node.DecodeIssuerPlan(raw)
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
	tx, err := admissionNumber(parts[0], 64)
	if err != nil {
		return hosting.Traffic{}, err
	}
	rx, err := admissionNumber(parts[1], 64)
	if err != nil {
		return hosting.Traffic{}, err
	}
	return hosting.Traffic{Tx: tx, Rx: rx}, nil
}

func admissionNumber(value string, bits int) (uint64, error) {
	number, err := strconv.ParseUint(value, 10, bits)
	if err != nil || strconv.FormatUint(number, 10) != value {
		return 0, errRuntimePlan
	}
	return number, nil
}
