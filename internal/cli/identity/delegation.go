package identity

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/storage"

	"connectrpc.com/connect"
)

const defaultDelegationTTL = 15 * time.Minute

type delegationView struct {
	Operation       string   `json:"operation"`
	ID              string   `json:"id,omitempty"`
	Delegator       string   `json:"delegator"`
	Application     string   `json:"application_principal"`
	TargetNode      string   `json:"target_node"`
	Actions         []string `json:"actions"`
	Scope           string   `json:"scope"`
	ResourceKind    string   `json:"resource_kind,omitempty"`
	ResourceID      string   `json:"resource_id,omitempty"`
	ResourceOwner   string   `json:"resource_owner,omitempty"`
	NotBefore       string   `json:"not_before"`
	NotAfter        string   `json:"not_after"`
	NoRedelegation  bool     `json:"no_redelegation"`
	ProtectedOutput string   `json:"protected_output"`
}

type delegationRevocationView struct {
	Operation       string `json:"operation"`
	DelegationID    string `json:"delegation_id"`
	RevocationID    string `json:"revocation_id,omitempty"`
	Delegator       string `json:"delegator"`
	Application     string `json:"application_principal"`
	TargetNode      string `json:"target_node"`
	RevokedAt       string `json:"revoked_at"`
	ProtectedOutput string `json:"protected_output,omitempty"`
}

func (c Command) runDelegation(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		delegationUsage(c.Renderer.Out)
		return 0
	}
	switch args[0] {
	case "issue":
		return c.runDelegationIssue(ctx, args[1:])
	case "revoke":
		return c.runDelegationRevoke(ctx, args[1:])
	case "import-revocation":
		return c.runDelegationRevocationImport(ctx, args[1:])
	default:
		return c.usageError(fmt.Sprintf("unknown delegation subcommand %q", args[0]))
	}
}

func (c Command) runDelegationIssue(ctx context.Context, args []string) int {
	flags := c.flagSet("identity delegation issue")
	var application, scopeName, kind, resourceID, signerPath, outputPath string
	var actions stringList
	validity := defaultDelegationTTL
	yes := false
	flags.StringVar(&application, "application", "", "Application installation Principal")
	flags.Var(&actions, "action", "delegated Application action (repeatable)")
	flags.StringVar(&scopeName, "scope", "", "resource scope: principal-owned or exact")
	flags.StringVar(&kind, "resource-kind", "", "exact resource kind")
	flags.StringVar(&resourceID, "resource-id", "", "exact canonical resource ID")
	flags.DurationVar(&validity, "valid-for", defaultDelegationTTL, "finite Delegation validity")
	flags.StringVar(&signerPath, "signer-file", "", "protected delegator device signer bundle")
	flags.StringVar(&outputPath, "out-file", "", "new protected Delegation artifact file")
	flags.BoolVar(&yes, "yes", false, "confirm the displayed consent")
	if !c.parseFlags(flags, args) {
		return 2
	}
	if c.sessions == nil {
		return c.Renderer.Failure(errAdministrationUnavailable)
	}
	node := c.sessions.TargetNodePrincipal()
	if _, err := identityprincipal.Parse(node); err != nil {
		return c.Renderer.Failure(errAdministrationUnavailable)
	}
	if _, err := identityprincipal.Parse(application); err != nil {
		return c.usageError("--application must be a canonical Principal")
	}
	if signerPath == "" {
		var err error
		signerPath, err = DefaultDeviceSignerPath()
		if err != nil {
			return c.Renderer.Failure(err)
		}
	}
	if !validArtifactPath(outputPath) {
		return c.usageError("--out-file must be an absolute path")
	}
	if _, err := os.Lstat(outputPath); err == nil || !os.IsNotExist(err) {
		return c.Renderer.Failure(errors.New("Delegation output already exists or cannot be inspected"))
	}
	signer, err := OpenDeviceFileSigner(signerPath)
	if err != nil {
		return c.Renderer.Failure(err)
	}
	delegator, err := signer.Principal(ctx)
	if err != nil || delegator == application {
		return c.Renderer.Failure(errors.New("Delegation signer is invalid for the Application"))
	}
	parsedActions, sortedActions, err := delegationActions(actions)
	if err != nil {
		return c.usageError(err.Error())
	}
	scope, err := delegationScope(scopeName, node, delegator, kind, resourceID)
	if err != nil {
		return c.usageError(err.Error())
	}
	if validity <= 0 || validity > identitycontract.MaxDelegationLifetime {
		return c.usageError("--valid-for is outside the supported Delegation lifetime")
	}
	now := c.now().UTC().Truncate(time.Second)
	notAfter := now.Add(validity)
	if notAfter.Nanosecond() != 0 || !notAfter.After(now) {
		return c.usageError("--valid-for is outside the supported Delegation lifetime")
	}
	view := delegationView{
		Operation: "delegation_issue", Delegator: delegator, Application: application, TargetNode: node,
		Actions: sortedActions, Scope: scopeName, ResourceKind: kind, ResourceID: resourceID,
		NotBefore: now.Format(time.RFC3339), NotAfter: notAfter.Format(time.RFC3339),
		NoRedelegation: true, ProtectedOutput: outputPath,
	}
	if scopeName == "principal-owned" || scopeName == "exact" {
		view.ResourceOwner = delegator
	}
	if err := c.confirmDelegation(view, yes); err != nil {
		return c.Renderer.Failure(err)
	}
	artifact, err := signer.SignDelegation(ctx, DelegationSpec{
		Delegatee: application,
		Audience:  identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: identitycontract.ProtocolMajor},
		Actions:   parsedActions, Scope: scope, NotBefore: now, NotAfter: notAfter,
	}, now)
	if err != nil {
		return c.Renderer.Failure(errors.New("Delegation signing failed"))
	}
	raw, err := artifact.MarshalBinary()
	if err != nil {
		return c.Renderer.Failure(errors.New("Delegation encoding failed"))
	}
	defer clear(raw)
	if err := storage.AtomicCreatePrivateFile(outputPath, raw); err != nil {
		return c.Renderer.Failure(errors.New("Delegation output could not be created"))
	}
	view.ID = artifact.ID()
	return c.renderDelegation(view)
}

func (c Command) runDelegationRevoke(ctx context.Context, args []string) int {
	flags := c.flagSet("identity delegation revoke")
	var delegationPath, signerPath, outputPath string
	yes := false
	flags.StringVar(&delegationPath, "delegation-file", "", "protected Delegation artifact file")
	flags.StringVar(&signerPath, "signer-file", "", "protected delegator device signer bundle")
	flags.StringVar(&outputPath, "out-file", "", "new protected Delegation revocation file")
	flags.BoolVar(&yes, "yes", false, "confirm the displayed revocation")
	if !c.parseFlags(flags, args) {
		return 2
	}
	if signerPath == "" {
		var err error
		signerPath, err = DefaultDeviceSignerPath()
		if err != nil {
			return c.Renderer.Failure(err)
		}
	}
	if !validArtifactPath(delegationPath) || !validArtifactPath(outputPath) {
		return c.usageError("--delegation-file and --out-file must be absolute paths")
	}
	if _, err := os.Lstat(outputPath); err == nil || !os.IsNotExist(err) {
		return c.Renderer.Failure(errors.New("Delegation revocation output already exists or cannot be inspected"))
	}
	raw, err := readBoundedArtifact(delegationPath)
	if err != nil {
		return c.Renderer.Failure(errors.New("Delegation artifact could not be read"))
	}
	defer clear(raw)
	delegation, err := identityaccess.ParseAndVerifyDelegation(raw, time.Time{})
	if err != nil {
		return c.Renderer.Failure(errors.New("Delegation artifact is invalid"))
	}
	payload := delegation.DelegationPayload()
	now := c.now().UTC().Truncate(time.Second)
	view := delegationRevocationView{Operation: "delegation_revoke", DelegationID: delegation.ID(), Delegator: payload.Delegator, Application: payload.Delegatee, TargetNode: payload.Audience.Node, RevokedAt: now.Format(time.RFC3339), ProtectedOutput: outputPath}
	if err := c.confirmDelegationRevocation(view, yes); err != nil {
		return c.Renderer.Failure(err)
	}
	signer, err := OpenDeviceFileSigner(signerPath)
	if err != nil {
		return c.Renderer.Failure(err)
	}
	revocation, err := signer.SignDelegationRevocation(ctx, delegation, now)
	if err != nil {
		return c.Renderer.Failure(errors.New("Delegation revocation signing failed"))
	}
	revocationRaw, err := revocation.MarshalBinary()
	if err != nil {
		return c.Renderer.Failure(errors.New("Delegation revocation encoding failed"))
	}
	defer clear(revocationRaw)
	if err := storage.AtomicCreatePrivateFile(outputPath, revocationRaw); err != nil {
		return c.Renderer.Failure(errors.New("Delegation revocation output could not be created"))
	}
	view.RevocationID = revocation.ID()
	return c.renderDelegationRevocation(view)
}

func (c Command) runDelegationRevocationImport(ctx context.Context, args []string) int {
	flags := c.flagSet("identity delegation import-revocation")
	var revocationPath string
	flags.StringVar(&revocationPath, "revocation-file", "", "protected Delegation revocation file")
	if !c.parseFlags(flags, args) {
		return 2
	}
	if c.sessions == nil {
		return c.Renderer.Failure(errAdministrationUnavailable)
	}
	if !validArtifactPath(revocationPath) {
		return c.usageError("--revocation-file must be an absolute path")
	}
	raw, err := readBoundedArtifact(revocationPath)
	if err != nil {
		return c.Renderer.Failure(errors.New("Delegation revocation could not be read"))
	}
	defer clear(raw)
	revocation, err := identityaccess.ParseAndVerifyDelegationRevocation(raw, c.now().UTC())
	if err != nil {
		return c.Renderer.Failure(errors.New("Delegation revocation is invalid"))
	}
	payload := revocation.DelegationRevocationPayload()
	if payload == nil || payload.Audience == nil || payload.Audience.Node != c.sessions.TargetNodePrincipal() {
		return c.Renderer.Failure(errors.New("Delegation revocation does not target the selected Node"))
	}
	service, err := c.sessions.PublicIdentityService()
	if err != nil {
		return c.Renderer.Failure(errAdministrationUnavailable)
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	requestRaw := append([]byte(nil), raw...)
	defer clear(requestRaw)
	response, err := service.ImportDelegationRevocation(callCtx, connect.NewRequest(&protocol.ImportDelegationRevocationRequest{Revocation: requestRaw}))
	if err != nil {
		return c.Renderer.Failure(err)
	}
	if response == nil || response.Msg == nil || hasUnknownMessage(response.Msg) || response.Msg.RevocationId != revocation.ID() || !validArtifactID(response.Msg.RevocationId, identitycontract.DelegationRevocationPrefix) {
		return c.Renderer.Failure(errInvalidIdentityResponse)
	}
	return c.renderDelegationRevocation(delegationRevocationView{Operation: "delegation_revocation_import", DelegationID: payload.TargetId, RevocationID: revocation.ID(), Delegator: payload.Delegator, Application: payload.Delegatee, TargetNode: payload.Audience.Node, RevokedAt: payload.RevokedAt.AsTime().Format(time.RFC3339)})
}

func validArtifactPath(path string) bool {
	return filepath.IsAbs(path) && !strings.ContainsRune(path, '\x00')
}

func readBoundedArtifact(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > identitycontract.MaxArtifactBytes {
		return nil, identityaccess.ErrInvalidArgument
	}
	raw, err := io.ReadAll(io.LimitReader(file, identitycontract.MaxArtifactBytes+1))
	if err != nil || !identitycontract.ValidArtifactSize(len(raw)) {
		clear(raw)
		return nil, identityaccess.ErrInvalidArgument
	}
	return raw, nil
}

func (c Command) confirmDelegationRevocation(view delegationRevocationView, yes bool) error {
	if !c.Renderer.JSON {
		c.Renderer.Header("Delegation revocation")
		c.Renderer.KV("delegation_id", view.DelegationID)
		c.Renderer.KV("delegator", view.Delegator)
		c.Renderer.KV("application_principal", view.Application)
		c.Renderer.KV("target_node", view.TargetNode)
		c.Renderer.KV("revoked_at", view.RevokedAt)
	}
	if yes {
		return nil
	}
	if c.Renderer.JSON || c.input == nil {
		return errConfirmationRequired
	}
	_, _ = fmt.Fprint(c.Renderer.Err, "Type yes to permanently revoke this Delegation: ")
	if !readExactConfirmation(c.input) {
		return errConfirmationRequired
	}
	return nil
}

func (c Command) renderDelegationRevocation(view delegationRevocationView) int {
	if c.Renderer.JSON {
		return c.renderJSON(view)
	}
	c.Renderer.KV("delegation_id", view.DelegationID)
	c.Renderer.KV("revocation_id", view.RevocationID)
	c.Renderer.KV("protected_output", view.ProtectedOutput)
	return 0
}

func delegationActions(values []string) ([]identityaccess.Action, []string, error) {
	if len(values) == 0 || len(values) > identitycontract.MaxActions {
		return nil, nil, fmt.Errorf("between 1 and %d --action values are required", identitycontract.MaxActions)
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	parsed := make([]identityaccess.Action, len(sorted))
	for index, value := range sorted {
		action, err := identityaccess.ParseAction(identityprotocol.Interface_INTERFACE_APPLICATION, value)
		if err != nil || index > 0 && value == sorted[index-1] {
			return nil, nil, errors.New("--action values must be known Application actions and unique")
		}
		parsed[index] = action
	}
	return parsed, sorted, nil
}

func delegationScope(name, node, owner, kind, resourceID string) (identityaccess.ResourceScope, error) {
	switch name {
	case "principal-owned":
		if kind != "" || resourceID != "" {
			return identityaccess.ResourceScope{}, errors.New("resource flags require --scope exact")
		}
		return identityaccess.ResourceScope{Kind: identityaccess.ScopePrincipalOwned, Owner: owner}, nil
	case "exact":
		if kind == "" {
			return identityaccess.ResourceScope{}, errors.New("--resource-kind is required for --scope exact")
		}
		resource, err := identityaccess.NewResourceRef(node, owner, kind, resourceID)
		if err != nil {
			return identityaccess.ResourceScope{}, errors.New("exact resource is invalid")
		}
		return identityaccess.ResourceScope{Kind: identityaccess.ScopeExact, Exact: resource}, nil
	default:
		return identityaccess.ResourceScope{}, errors.New("--scope must be principal-owned or exact")
	}
}

func (c Command) confirmDelegation(view delegationView, yes bool) error {
	if !c.Renderer.JSON {
		c.renderDelegationDetails(view)
	}
	if yes {
		return nil
	}
	if c.Renderer.JSON || c.input == nil {
		return errConfirmationRequired
	}
	_, _ = fmt.Fprint(c.Renderer.Err, "Type yes to sign this non-redelegable Delegation: ")
	if !readExactConfirmation(c.input) {
		return errConfirmationRequired
	}
	return nil
}

func readExactConfirmation(input io.Reader) bool {
	line, err := bufio.NewReader(io.LimitReader(input, 32)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r") == "yes"
}

func (c Command) renderDelegationDetails(view delegationView) {
	c.Renderer.Header("Delegation consent")
	c.Renderer.KV("delegator", view.Delegator)
	c.Renderer.KV("application_principal", view.Application)
	c.Renderer.KV("target_node", view.TargetNode)
	c.Renderer.KV("actions", strings.Join(view.Actions, ","))
	c.Renderer.KV("scope", view.Scope)
	if view.Scope == "exact" {
		c.Renderer.KV("resource_kind", view.ResourceKind)
		c.Renderer.KV("resource_id", view.ResourceID)
	}
	c.Renderer.KV("resource_owner", view.ResourceOwner)
	c.Renderer.KV("not_before", view.NotBefore)
	c.Renderer.KV("not_after", view.NotAfter)
	c.Renderer.KV("redelegation", "forbidden")
}

func (c Command) renderDelegation(view delegationView) int {
	if c.Renderer.JSON {
		return c.renderJSON(view)
	}
	c.Renderer.KV("delegation_id", view.ID)
	c.Renderer.KV("protected_output", view.ProtectedOutput)
	return 0
}

func delegationUsage(writer io.Writer) {
	_, _ = fmt.Fprintln(writer, "Usage: ardentsctl [global flags] identity delegation <issue|revoke|import-revocation> [flags]")
}
