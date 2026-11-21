package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

// runNodePreparePlan codifies the platform-led node-role provisioning recipe: it
// writes the role's exact runtime invocation declaration from operator inputs.
// It is pure configuration preparation (ADR-0125 decision 6 keeps validation with
// the owner). It performs no State acceptance, no hosting initialization and no
// root creation; those stay with network accept-*, hosting initialize and the
// runtime. Every refusal precedes the single output write.
func runNodePreparePlan(ctx context.Context, args []string, out io.Writer) int {
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
	for _, name := range []string{"network", "participation-root", "spend-root", "hosting-root", "certificate", "private-key", "output"} {
		if !absoluteAdmissionPath(flags[name]) {
			return 2
		}
	}
	networkRaw, err := readBounded(flags["network"], 16<<10)
	if err != nil {
		return 2
	}
	network, err := node.DecodeNodeNetwork(networkRaw)
	if err != nil {
		return 2
	}
	nodeID, err := admissionHex(flags["node-id"], 32)
	if err != nil {
		return 2
	}
	var id [32]byte
	copy(id[:], nodeID)
	work, err := preparePlanTraffic(flags["work"])
	if err != nil {
		return 2
	}
	termination, err := preparePlanTraffic(flags["termination"])
	if err != nil {
		return 2
	}
	wait, err := admissionNumber(flags["retirement-wait-ms"], 32)
	if err != nil || wait == 0 || wait > 5000 {
		return 2
	}
	profile, roots, issuanceRaw, bootstrap, err := preparePlanRole(role, flags)
	if err != nil {
		return 2
	}
	authority := (*networkAuthorityPlan)(&network)
	if !validAdmissionAuthority("", authority, roots...) || !independentRouteRoots(append([]string{authority.Root}, roots...)...) {
		return 2
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
		return 2
	}
	if err := preparePlanGrammar(role, raw); err != nil {
		return 2
	}
	f, err := os.OpenFile(flags["output"], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return 2
	}
	defer f.Close()
	if _, err := f.Write(append(raw, '\n')); err != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
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

// preparePlanRole selects the role's process profile and writable-root list and
// acquires the Issuer initialize-results sub-plan. The per-role policy stays here
// in command composition; the shared codec supplies no universal role policy.
func preparePlanRole(role string, flags map[string]string) (profile string, roots []string, issuanceRaw []byte, bootstrap hosting.Traffic, err error) {
	bad := errors.New("invalid prepare-plan role input")
	roots = []string{flags["participation-root"], flags["spend-root"], flags["hosting-root"]}
	switch role {
	case "forwarding":
		profile = processresource.Profile
	case "join":
		profile = processresource.JoinProfile
	case "descriptor":
		if !absoluteAdmissionPath(flags["descriptor-root"]) {
			return "", nil, nil, bootstrap, bad
		}
		profile = processresource.DescriptorProfile
		roots = append(roots, flags["descriptor-root"])
	case "introduction":
		if !absoluteAdmissionPath(flags["introduction-root"]) {
			return "", nil, nil, bootstrap, bad
		}
		profile = processresource.IntroductionProfile
		roots = append(roots, flags["introduction-root"])
	case "issuer":
		if !absoluteAdmissionPath(flags["issuance"]) {
			return "", nil, nil, bootstrap, bad
		}
		profile = processresource.IssuerProfile
		issuanceRaw, err = readBounded(flags["issuance"], 16<<10)
		if err != nil {
			return "", nil, nil, bootstrap, bad
		}
		prepared, e := decodeIssuanceResultPlan(issuanceRaw, "initialize-results")
		if e != nil {
			return "", nil, nil, bootstrap, bad
		}
		bootstrap, err = preparePlanTraffic(flags["bootstrap-work"])
		if err != nil {
			return "", nil, nil, bootstrap, bad
		}
		roots = append(roots, prepared.Admission.Root, prepared.Keys.Root, prepared.Root)
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
