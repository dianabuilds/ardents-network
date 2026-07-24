package identity

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"flag"
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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultGrantTTL      = identitycontract.DefaultGrantLifetime
	maxBootstrapFileSize = 256
)

var (
	errAdministrationUnavailable = errors.New("Principal administration is not configured")
	errInvalidIdentityResponse   = errors.New("identity service returned an invalid response")
	errInvalidBootstrapTicket    = errors.New("bootstrap ticket file is invalid")
	errConfirmationRequired      = errors.New("explicit confirmation is required")
)

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ",") }
func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

type grantView struct {
	ID            string   `json:"id"`
	Subject       string   `json:"subject_principal"`
	TargetNode    string   `json:"target_node"`
	Actions       []string `json:"actions"`
	Scope         string   `json:"scope"`
	ResourceKind  string   `json:"resource_kind,omitempty"`
	ResourceID    string   `json:"resource_id,omitempty"`
	ResourceOwner string   `json:"resource_owner,omitempty"`
	NotBefore     string   `json:"not_before"`
	NotAfter      string   `json:"not_after"`
	Revoked       bool     `json:"revoked"`
}

type mutationView struct {
	Operation     string   `json:"operation"`
	Principal     string   `json:"principal"`
	TargetNode    string   `json:"target_node"`
	Actions       []string `json:"actions,omitempty"`
	Scope         string   `json:"scope,omitempty"`
	ResourceKind  string   `json:"resource_kind,omitempty"`
	ResourceID    string   `json:"resource_id,omitempty"`
	ResourceOwner string   `json:"resource_owner,omitempty"`
	NotBefore     string   `json:"not_before,omitempty"`
	NotAfter      string   `json:"not_after,omitempty"`
	DeviceID      string   `json:"device_id,omitempty"`
	GrantID       string   `json:"grant_id,omitempty"`
	ResultID      string   `json:"result_id"`
	RequestID     string   `json:"request_id,omitempty"`
}

type applicationTicketView struct {
	Operation       string   `json:"operation"`
	Principal       string   `json:"application_principal"`
	TargetNode      string   `json:"target_node"`
	Actions         []string `json:"initial_actions"`
	ExpiresAt       string   `json:"expires_at"`
	ProtectedOutput string   `json:"protected_output"`
}

func (c Command) runApplicationTicket(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		_, _ = fmt.Fprintln(c.Renderer.Out, "Usage: ardentsctl [global flags] identity application-ticket issue [flags]")
		return 0
	}
	if args[0] != "issue" {
		return c.usageError(fmt.Sprintf("unknown application-ticket subcommand %q", args[0]))
	}
	flags := c.flagSet("identity application-ticket issue")
	var principal, outputPath string
	var actions stringList
	yes := false
	flags.StringVar(&principal, "principal", "", "Application installation Principal")
	flags.Var(&actions, "action", "initial Application action (repeatable)")
	flags.StringVar(&outputPath, "out-file", "", "new protected one-use Application enrollment ticket file")
	flags.BoolVar(&yes, "yes", false, "confirm the displayed mutation")
	if !c.parseFlags(flags, args[1:]) {
		return 2
	}
	if _, err := identityprincipal.Parse(principal); err != nil {
		return c.usageError("--principal must be canonical")
	}
	if !filepath.IsAbs(outputPath) || strings.ContainsRune(outputPath, '\x00') {
		return c.usageError("--out-file must be an absolute path")
	}
	if _, err := os.Lstat(outputPath); err == nil || !os.IsNotExist(err) {
		return c.Renderer.Failure(errors.New("Application enrollment ticket output already exists or cannot be inspected"))
	}
	if len(actions) == 0 || len(actions) > 2 {
		return c.usageError("one or two --action values are required")
	}
	sorted := append([]string(nil), actions...)
	sort.Strings(sorted)
	for index, action := range sorted {
		if _, err := identityaccess.ParseAction(identityprotocol.Interface_INTERFACE_APPLICATION, action); err != nil || index > 0 && action == sorted[index-1] {
			return c.usageError("--action values must be known Application actions and unique")
		}
	}
	if c.sessions == nil {
		return c.Renderer.Failure(errAdministrationUnavailable)
	}
	view := applicationTicketView{Operation: "application_enrollment_ticket_issue", Principal: principal, TargetNode: c.sessions.TargetNodePrincipal(), Actions: sorted, ProtectedOutput: outputPath}
	if err := c.confirmApplicationTicket(view, yes); err != nil {
		return c.Renderer.Failure(err)
	}
	service, err := c.protectedService()
	if err != nil {
		return c.Renderer.Failure(err)
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := service.IssueApplicationEnrollmentTicket(callCtx, connect.NewRequest(&protocol.IssueApplicationEnrollmentTicketRequest{ApplicationPrincipalId: principal, InitialActions: sorted}))
	if err != nil {
		return c.Renderer.Failure(err)
	}
	if response == nil || response.Msg == nil {
		return c.Renderer.Failure(errInvalidIdentityResponse)
	}
	defer zero(response.Msg.ApplicationEnrollmentTicket)
	now := c.now().UTC()
	if hasUnknownMessage(response.Msg) || len(response.Msg.ApplicationEnrollmentTicket) != identitycontract.ApplicationEnrollmentTicketBytes || response.Msg.ExpiresAt == nil || !response.Msg.ExpiresAt.IsValid() || response.Msg.ExpiresAt.Nanos != 0 {
		return c.Renderer.Failure(errInvalidIdentityResponse)
	}
	expires := response.Msg.ExpiresAt.AsTime()
	if !now.Before(expires) || expires.After(now.Add(identitycontract.ApplicationEnrollmentTicketLifetime)) {
		return c.Renderer.Failure(errInvalidIdentityResponse)
	}
	encodedText, ok := identitycontract.EncodeApplicationEnrollmentTicket(response.Msg.ApplicationEnrollmentTicket)
	if !ok {
		return c.Renderer.Failure(errInvalidIdentityResponse)
	}
	encoded := []byte(encodedText)
	defer zero(encoded)
	if err := storage.AtomicCreatePrivateFile(outputPath, encoded); err != nil {
		return c.Renderer.Failure(errors.New("write protected Application enrollment ticket file"))
	}
	view.ExpiresAt = expires.UTC().Format(time.RFC3339)
	if c.Renderer.JSON {
		return c.renderJSON(view)
	}
	c.Renderer.KV("application_principal", view.Principal)
	c.Renderer.KV("target_node", view.TargetNode)
	c.Renderer.KV("expires_at", view.ExpiresAt)
	c.Renderer.KV("protected_output", view.ProtectedOutput)
	return 0
}

func (c Command) confirmApplicationTicket(view applicationTicketView, yes bool) error {
	if c.Renderer.JSON || yes {
		return nil
	}
	if c.input == nil {
		return errConfirmationRequired
	}
	c.Renderer.Header("Identity mutation")
	c.Renderer.KV("operation", view.Operation)
	c.Renderer.KV("application_principal", view.Principal)
	c.Renderer.KV("target_node", view.TargetNode)
	c.Renderer.KV("initial_actions", strings.Join(view.Actions, ","))
	_, _ = fmt.Fprint(c.Renderer.Err, "Type yes to continue: ")
	line, err := bufio.NewReader(io.LimitReader(c.input, 32)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return errConfirmationRequired
	}
	if strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r") != "yes" {
		return errConfirmationRequired
	}
	return nil
}

func (c Command) runEnroll(ctx context.Context, args []string) int {
	flags := c.flagSet("identity enroll")
	var ticketPath, rootPath, devicePath, requestID string
	flags.StringVar(&ticketPath, "bootstrap-ticket-file", "", "protected one-use Bootstrap Ticket file")
	flags.StringVar(&rootPath, "root-signer-file", "", "protected Principal root signer bundle")
	flags.StringVar(&devicePath, "device-signer-file", "", "protected device signer bundle")
	flags.StringVar(&requestID, "request-id", "", "idempotency key for subsequent enrollment")
	if !c.parseFlags(flags, args) {
		return 2
	}
	if c.sessions == nil {
		return c.Renderer.Failure(errAdministrationUnavailable)
	}
	var bootstrapTicket []byte
	if ticketPath != "" {
		if requestID != "" {
			return c.usageError("--request-id is not accepted with --bootstrap-ticket-file")
		}
		var ticketErr error
		bootstrapTicket, ticketErr = readBootstrapTicket(ticketPath)
		if ticketErr != nil {
			return c.Renderer.Failure(ticketErr)
		}
		defer zero(bootstrapTicket)
	}
	var err error
	if rootPath == "" {
		rootPath, err = DefaultPrincipalSignerPath()
		if err != nil {
			return c.Renderer.Failure(err)
		}
	}
	if devicePath == "" {
		devicePath, err = DefaultDeviceSignerPath()
		if err != nil {
			return c.Renderer.Failure(err)
		}
	}
	root, err := OpenRootFileSigner(rootPath)
	if err != nil {
		return c.Renderer.Failure(err)
	}
	device, err := OpenDeviceFileSigner(devicePath)
	if err != nil {
		return c.Renderer.Failure(err)
	}
	principal, credential, rootPublic, err := enrollmentMaterial(ctx, root, device)
	if err != nil {
		return c.Renderer.Failure(err)
	}
	defer zero(credential)
	public, err := c.sessions.PublicIdentityService()
	if err != nil {
		return c.Renderer.Failure(errAdministrationUnavailable)
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	begin, err := public.BeginAuthentication(callCtx, connect.NewRequest(&protocol.BeginAuthenticationRequest{PrincipalId: principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF}))
	if err != nil {
		return c.Renderer.Failure(err)
	}
	challenge, err := validateEnrollmentChallenge(begin.Msg, principal, c.sessions.TargetNodePrincipal(), c.now())
	if err != nil {
		return c.Renderer.Failure(err)
	}
	signature, err := root.SignEnrollmentChallenge(callCtx, challenge)
	if err != nil {
		return c.Renderer.Failure(err)
	}
	complete, err := public.CompleteAuthentication(callCtx, connect.NewRequest(&protocol.CompleteAuthenticationRequest{ChallengeId: append([]byte(nil), challenge.ID[:]...), PrincipalId: principal, RootPublicKey: rootPublic, Signature: signature}))
	defer zero(signature)
	if err != nil {
		return c.Renderer.Failure(err)
	}
	proof, err := validateEnrollmentProof(complete.Msg)
	if err != nil {
		return c.Renderer.Failure(err)
	}
	defer zero(proof)
	fields, err := identityaccess.ChallengeFields(challenge)
	if err != nil {
		return c.Renderer.Failure(errInvalidIdentityResponse)
	}
	var enrolled string
	if ticketPath != "" {
		response, callErr := public.EnrollFirstPrincipal(callCtx, connect.NewRequest(&protocol.EnrollFirstPrincipalRequest{BootstrapTicket: bootstrapTicket, Challenge: fields, EnrollmentProof: proof, RootPublicKey: rootPublic, Credential: credential}))
		if callErr != nil {
			return c.Renderer.Failure(callErr)
		}
		if response.Msg == nil || hasUnknownMessage(response.Msg) || response.Msg.PrincipalId != principal {
			return c.Renderer.Failure(errInvalidIdentityResponse)
		}
		removeTicket := c.removeTicket
		if removeTicket == nil {
			removeTicket = removeConsumedBootstrapTicket
		}
		if err := removeTicket(ticketPath); err != nil {
			return c.Renderer.Failure(err)
		}
		enrolled = response.Msg.PrincipalId
	} else {
		requestID, err = c.adminRequestID(requestID)
		if err != nil {
			return c.usageError(err.Error())
		}
		protected, serviceErr := c.sessions.ProtectedIdentityService()
		if serviceErr != nil {
			return c.Renderer.Failure(errAdministrationUnavailable)
		}
		request := connect.NewRequest(&protocol.EnrollPrincipalRequest{RequestId: requestID, Challenge: fields, EnrollmentProof: proof, RootPublicKey: rootPublic, Credential: credential})
		response, callErr := retryAdminCall(callCtx, func() (*connect.Response[protocol.EnrollPrincipalResponse], error) {
			return protected.EnrollPrincipal(callCtx, request)
		})
		if callErr != nil {
			return c.Renderer.Failure(adminCallError(requestID, callErr))
		}
		if response.Msg == nil || hasUnknownMessage(response.Msg) || response.Msg.PrincipalId != principal {
			return c.Renderer.Failure(errInvalidIdentityResponse)
		}
		enrolled = response.Msg.PrincipalId
	}
	return c.renderEnrollment(enrolled, c.sessions.TargetNodePrincipal(), requestID, ticketPath != "")
}

func enrollmentMaterial(ctx context.Context, root EnrollmentSigner, device SessionSigner) (string, []byte, []byte, error) {
	rootPrincipal, err := root.Principal(ctx)
	if err != nil {
		return "", nil, nil, err
	}
	devicePrincipal, err := device.Principal(ctx)
	if err != nil || devicePrincipal != rootPrincipal {
		return "", nil, nil, ErrSignerFileInvalid
	}
	credential, err := device.Credential(ctx)
	if err != nil || credential == nil {
		return "", nil, nil, ErrSignerFileInvalid
	}
	payload := credential.KeyCredentialPayload()
	if payload == nil || payload.Subject != rootPrincipal || len(payload.RootPublicKey) != ed25519.PublicKeySize {
		return "", nil, nil, ErrSignerFileInvalid
	}
	raw, err := credential.MarshalBinary()
	if err != nil {
		return "", nil, nil, ErrSignerFileInvalid
	}
	return rootPrincipal, raw, append([]byte(nil), payload.RootPublicKey...), nil
}

func validateEnrollmentChallenge(response *protocol.BeginAuthenticationResponse, principal, node string, now time.Time) (identityaccess.Challenge, error) {
	if response == nil || hasUnknownMessage(response) || response.Challenge == nil {
		return identityaccess.Challenge{}, errInvalidIdentityResponse
	}
	challenge, err := identityaccess.ParseChallengeFields(response.Challenge)
	if err != nil || challenge.Principal != principal || challenge.Binding.Audience.Node != node || challenge.Binding.Audience.Interface != identityprotocol.Interface_INTERFACE_OPERATOR || challenge.Binding.Audience.ProtocolMajor != identitycontract.ProtocolMajor || challenge.Binding.TransportProfile != identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1 || challenge.Binding.PeerBinding == [32]byte{} || challenge.Purpose != identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF {
		return identityaccess.Challenge{}, errInvalidIdentityResponse
	}
	now = now.UTC()
	if now.Before(challenge.IssuedAt) || !now.Before(challenge.ExpiresAt) {
		return identityaccess.Challenge{}, errInvalidIdentityResponse
	}
	return challenge, nil
}

func validateEnrollmentProof(response *protocol.CompleteAuthenticationResponse) ([]byte, error) {
	if response == nil || hasUnknownMessage(response) || len(response.EnrollmentProof) != len(identityaccess.EnrollmentProof{}) || len(response.SessionSecret) != 0 || response.SessionId != "" || response.ExpiresAt != nil {
		return nil, errInvalidIdentityResponse
	}
	return append([]byte(nil), response.EnrollmentProof...), nil
}

func readBootstrapTicket(path string) ([]byte, error) {
	raw, found, err := storage.ReadStrictPrivateFileBounded(path, maxBootstrapFileSize)
	if err != nil || !found {
		return nil, errInvalidBootstrapTicket
	}
	defer zero(raw)
	if bytes.HasSuffix(raw, []byte("\r\n")) {
		raw = raw[:len(raw)-2]
	} else if bytes.HasSuffix(raw, []byte("\n")) {
		raw = raw[:len(raw)-1]
	}
	decoded, err := base64.RawURLEncoding.DecodeString(string(raw))
	if err != nil || len(decoded) != identitycontract.BootstrapTicketBytes || base64.RawURLEncoding.EncodeToString(decoded) != string(raw) {
		zero(decoded)
		return nil, errInvalidBootstrapTicket
	}
	return decoded, nil
}

func removeConsumedBootstrapTicket(path string) error {
	if err := storage.ValidatePrivateDir(filepath.Dir(path)); err != nil {
		return errors.New("Principal enrolled but Bootstrap Ticket cleanup failed")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("Principal enrolled but Bootstrap Ticket cleanup failed")
	}
	if err := os.Remove(path); err != nil {
		return errors.New("Principal enrolled but Bootstrap Ticket cleanup failed")
	}
	return nil
}

func (c Command) runGrant(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		grantUsage(c.Renderer.Out)
		return 0
	}
	switch args[0] {
	case "list":
		return c.runGrantList(ctx, args[1:])
	case "issue":
		return c.runGrantIssue(ctx, args[1:])
	case "revoke":
		return c.runGrantRevoke(ctx, args[1:])
	default:
		return c.usageError(fmt.Sprintf("unknown grant subcommand %q", args[0]))
	}
}

func (c Command) runGrantList(ctx context.Context, args []string) int {
	flags := c.flagSet("identity grant list")
	var subject string
	flags.StringVar(&subject, "subject", "", "enrolled Principal")
	if !c.parseFlags(flags, args) {
		return 2
	}
	if _, err := identityprincipal.Parse(subject); err != nil {
		return c.usageError("--subject must be a canonical Principal")
	}
	items, err := c.listGrants(ctx, subject)
	if err != nil {
		return c.Renderer.Failure(err)
	}
	if c.Renderer.JSON {
		return c.renderJSON(struct {
			Grants []grantView `json:"grants"`
		}{Grants: items})
	}
	for _, item := range items {
		c.renderGrant(item)
	}
	return 0
}

func (c Command) runGrantIssue(ctx context.Context, args []string) int {
	flags := c.flagSet("identity grant issue")
	var subject, scopeName, kind, resourceID, owner, requestID string
	var actions stringList
	validity, yes := defaultGrantTTL, false
	flags.StringVar(&subject, "subject", "", "enrolled Principal")
	flags.Var(&actions, "action", "exact Operator action (repeatable)")
	flags.StringVar(&scopeName, "scope", "node", "node or exact")
	flags.StringVar(&kind, "resource-kind", "", "exact ResourceKind")
	flags.StringVar(&resourceID, "resource-id", "", "exact canonical resource ID")
	flags.StringVar(&owner, "resource-owner", "", "exact resource owner Principal when required")
	flags.DurationVar(&validity, "valid-for", defaultGrantTTL, "finite grant validity")
	flags.StringVar(&requestID, "request-id", "", "idempotency key")
	flags.BoolVar(&yes, "yes", false, "confirm the displayed mutation")
	if !c.parseFlags(flags, args) {
		return 2
	}
	proposal, view, err := c.grantProposal(subject, actions, scopeName, kind, resourceID, owner, validity)
	if err != nil {
		return c.usageError(err.Error())
	}
	requestID, err = c.adminRequestID(requestID)
	if err != nil {
		return c.usageError(err.Error())
	}
	if err := c.confirmGrant(view, requestID, yes); err != nil {
		return c.Renderer.Failure(err)
	}
	service, err := c.protectedService()
	if err != nil {
		return c.Renderer.Failure(err)
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	request := connect.NewRequest(&protocol.IssueAccessGrantRequest{RequestId: requestID, Proposal: proposal})
	response, err := retryAdminCall(callCtx, func() (*connect.Response[protocol.IssueAccessGrantResponse], error) {
		return service.IssueAccessGrant(callCtx, request)
	})
	if err != nil {
		return c.Renderer.Failure(adminCallError(requestID, err))
	}
	if response.Msg == nil || hasUnknownMessage(response.Msg) || !validArtifactID(response.Msg.GrantId, identitycontract.AccessGrantPrefix) {
		return c.Renderer.Failure(errInvalidIdentityResponse)
	}
	view.ID = response.Msg.GrantId
	return c.renderMutation(mutationFromGrant("grant_issue", view, requestID, response.Msg.GrantId))
}

func (c Command) grantProposal(subject string, actions []string, scopeName, kind, resourceID, owner string, validity time.Duration) (*protocol.AccessGrantProposal, grantView, error) {
	if c.sessions == nil {
		return nil, grantView{}, errAdministrationUnavailable
	}
	if _, err := identityprincipal.Parse(subject); err != nil {
		return nil, grantView{}, errors.New("--subject must be a canonical Principal")
	}
	if len(actions) == 0 || len(actions) > identitycontract.MaxActions {
		return nil, grantView{}, fmt.Errorf("between 1 and %d --action values are required", identitycontract.MaxActions)
	}
	sorted := append([]string(nil), actions...)
	sort.Strings(sorted)
	for index, value := range sorted {
		if _, err := identityaccess.ParseAction(identityprotocol.Interface_INTERFACE_OPERATOR, value); err != nil || index > 0 && value == sorted[index-1] {
			return nil, grantView{}, errors.New("--action values must be known and unique")
		}
	}
	node := c.sessions.TargetNodePrincipal()
	if _, err := identityprincipal.Parse(node); err != nil {
		return nil, grantView{}, errAdministrationUnavailable
	}
	var domainScope identityaccess.ResourceScope
	switch scopeName {
	case "node":
		if kind != "" || resourceID != "" || owner != "" {
			return nil, grantView{}, errors.New("resource flags require --scope exact")
		}
		domainScope = identityaccess.ResourceScope{Kind: identityaccess.ScopeNode, Exact: identityaccess.ResourceRef{Node: node}}
	case "exact":
		if !visibleASCII(resourceID) {
			return nil, grantView{}, errors.New("exact resource ID must contain visible ASCII bytes")
		}
		parsedOwner, err := identityaccess.ParseResourceOwner(owner)
		if err != nil {
			return nil, grantView{}, errors.New("exact resource owner must be a canonical Principal")
		}
		resource, err := identityaccess.NewResourceRef(node, parsedOwner, kind, resourceID)
		if err != nil {
			return nil, grantView{}, errors.New("exact resource is invalid")
		}
		domainScope = identityaccess.ResourceScope{Kind: identityaccess.ScopeExact, Exact: resource}
	default:
		return nil, grantView{}, errors.New("--scope must be node or exact")
	}
	if validity <= 0 || validity > identitycontract.MaxGrantLifetime {
		return nil, grantView{}, errors.New("--valid-for is outside the supported grant lifetime")
	}
	notBefore := c.now().UTC().Truncate(time.Second)
	notAfter := notBefore.Add(validity).UTC().Truncate(time.Second)
	if !notAfter.After(notBefore) {
		return nil, grantView{}, errors.New("--valid-for is outside the supported grant lifetime")
	}
	wireScope, err := identityaccess.ResourceScopeFields(domainScope, identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: identitycontract.ProtocolMajor})
	if err != nil {
		return nil, grantView{}, errors.New("scope is not allowed on the Operator interface")
	}
	view := grantView{Subject: subject, TargetNode: node, Actions: sorted, Scope: scopeName, ResourceKind: kind, ResourceID: resourceID, ResourceOwner: owner, NotBefore: notBefore.Format(time.RFC3339), NotAfter: notAfter.Format(time.RFC3339)}
	return &protocol.AccessGrantProposal{SubjectPrincipalId: subject, Actions: sorted, Scope: wireScope, NotBefore: timestamppb.New(notBefore), NotAfter: timestamppb.New(notAfter)}, view, nil
}

func (c Command) runGrantRevoke(ctx context.Context, args []string) int {
	flags := c.flagSet("identity grant revoke")
	var subject, grantID, requestID string
	yes := false
	flags.StringVar(&subject, "subject", "", "grant subject Principal")
	flags.StringVar(&grantID, "grant-id", "", "Access Grant ID")
	flags.StringVar(&requestID, "request-id", "", "idempotency key")
	flags.BoolVar(&yes, "yes", false, "confirm the displayed mutation")
	if !c.parseFlags(flags, args) {
		return 2
	}
	if _, err := identityprincipal.Parse(subject); err != nil || !validArtifactID(grantID, identitycontract.AccessGrantPrefix) {
		return c.usageError("--subject and --grant-id must be canonical")
	}
	items, err := c.listGrants(ctx, subject)
	if err != nil {
		return c.Renderer.Failure(err)
	}
	var selected *grantView
	for index := range items {
		if items[index].ID == grantID {
			selected = &items[index]
			break
		}
	}
	if selected == nil || selected.Revoked {
		return c.Renderer.Failure(errors.New("active Access Grant was not found"))
	}
	requestID, err = c.adminRequestID(requestID)
	if err != nil {
		return c.usageError(err.Error())
	}
	if err := c.confirmGrant(*selected, requestID, yes); err != nil {
		return c.Renderer.Failure(err)
	}
	service, err := c.protectedService()
	if err != nil {
		return c.Renderer.Failure(err)
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	request := connect.NewRequest(&protocol.RevokeAccessGrantRequest{RequestId: requestID, GrantId: grantID})
	response, err := retryAdminCall(callCtx, func() (*connect.Response[protocol.RevokeAccessGrantResponse], error) {
		return service.RevokeAccessGrant(callCtx, request)
	})
	if err != nil {
		return c.Renderer.Failure(adminCallError(requestID, err))
	}
	if response.Msg == nil || hasUnknownMessage(response.Msg) || !validArtifactID(response.Msg.RevocationId, identitycontract.AccessGrantRevocationPrefix) {
		return c.Renderer.Failure(errInvalidIdentityResponse)
	}
	return c.renderMutation(mutationFromGrant("grant_revoke", *selected, requestID, response.Msg.RevocationId))
}

func (c Command) listGrants(ctx context.Context, subject string) ([]grantView, error) {
	service, err := c.protectedService()
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := service.ListAccessGrants(callCtx, connect.NewRequest(&protocol.ListAccessGrantsRequest{SubjectPrincipalId: subject}))
	if err != nil {
		return nil, err
	}
	if response.Msg == nil || hasUnknownMessage(response.Msg) {
		return nil, errInvalidIdentityResponse
	}
	items := make([]grantView, len(response.Msg.Grants))
	seen := make(map[string]struct{}, len(items))
	for index, item := range response.Msg.Grants {
		view, parseErr := parseGrantView(item, c.sessions.TargetNodePrincipal(), subject)
		if parseErr != nil {
			return nil, errInvalidIdentityResponse
		}
		if _, duplicate := seen[view.ID]; duplicate {
			return nil, errInvalidIdentityResponse
		}
		seen[view.ID] = struct{}{}
		items[index] = view
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func parseGrantView(item *protocol.AccessGrantMetadata, node, subject string) (grantView, error) {
	if _, err := identityprincipal.Parse(node); err != nil {
		return grantView{}, errInvalidIdentityResponse
	}
	if item == nil || hasUnknownMessage(item) || item.SubjectPrincipalId != subject || !validArtifactID(item.Id, identitycontract.AccessGrantPrefix) || item.NotBefore == nil || item.NotAfter == nil || !item.NotBefore.IsValid() || !item.NotAfter.IsValid() {
		return grantView{}, errInvalidIdentityResponse
	}
	notBefore, notAfter := item.NotBefore.AsTime(), item.NotAfter.AsTime()
	lower := time.Unix(identitycontract.LowerTimestampUnix, 0).UTC()
	upper := time.Unix(identitycontract.UpperTimestampUnix, 0).UTC()
	if notBefore.Nanosecond() != 0 || notAfter.Nanosecond() != 0 || notBefore.Before(lower) || !notBefore.Before(upper) || !notAfter.Before(upper) || !notAfter.After(notBefore) || notAfter.Sub(notBefore) > identitycontract.MaxGrantLifetime {
		return grantView{}, errInvalidIdentityResponse
	}
	actions := append([]string(nil), item.Actions...)
	if len(actions) == 0 || !sort.StringsAreSorted(actions) {
		return grantView{}, errInvalidIdentityResponse
	}
	actionSurface := identityprotocol.Interface_INTERFACE_OPERATOR
	if _, err := identityaccess.ParseAction(actionSurface, actions[0]); err != nil {
		actionSurface = identityprotocol.Interface_INTERFACE_APPLICATION
	}
	for index, action := range actions {
		if _, err := identityaccess.ParseAction(actionSurface, action); err != nil || index > 0 && action == actions[index-1] {
			return grantView{}, errInvalidIdentityResponse
		}
	}
	scope, err := identityaccess.ParseResourceScope(item.Scope, node)
	if err != nil {
		return grantView{}, errInvalidIdentityResponse
	}
	view := grantView{ID: item.Id, Subject: subject, TargetNode: node, Actions: actions, NotBefore: notBefore.UTC().Format(time.RFC3339), NotAfter: notAfter.UTC().Format(time.RFC3339), Revoked: item.Revoked}
	if scope.Kind == identityaccess.ScopeNode {
		view.Scope = "node"
	} else if scope.Kind == identityaccess.ScopeExact {
		if scope.Exact.Node != node || !visibleASCII(scope.Exact.ID) {
			return grantView{}, errInvalidIdentityResponse
		}
		view.Scope, view.ResourceKind, view.ResourceID, view.ResourceOwner = "exact", string(scope.Exact.Kind), scope.Exact.ID, scope.Exact.Owner.String()
	} else {
		return grantView{}, errInvalidIdentityResponse
	}
	return view, nil
}

func (c Command) runDeviceRevoke(ctx context.Context, args []string) int {
	flags := c.flagSet("identity device revoke")
	var principal, deviceID, requestID string
	yes := false
	flags.StringVar(&principal, "principal", "", "device owner Principal")
	flags.StringVar(&deviceID, "device-id", "", "DeviceID to revoke")
	flags.StringVar(&requestID, "request-id", "", "idempotency key")
	flags.BoolVar(&yes, "yes", false, "confirm the displayed mutation")
	if !c.parseFlags(flags, args) {
		return 2
	}
	if _, err := identityprincipal.Parse(principal); err != nil {
		return c.usageError("--principal must be canonical")
	}
	if _, err := identityprincipal.ParseDeviceID(deviceID); err != nil {
		return c.usageError("--device-id must be canonical")
	}
	if c.sessions == nil {
		return c.Renderer.Failure(errAdministrationUnavailable)
	}
	node := c.sessions.TargetNodePrincipal()
	if _, err := identityprincipal.Parse(node); err != nil {
		return c.Renderer.Failure(errAdministrationUnavailable)
	}
	requestID, err := c.adminRequestID(requestID)
	if err != nil {
		return c.usageError(err.Error())
	}
	view := mutationView{Operation: "device_revoke", Principal: principal, TargetNode: node, DeviceID: deviceID, RequestID: requestID}
	if err := c.confirmMutation(view, yes); err != nil {
		return c.Renderer.Failure(err)
	}
	service, err := c.protectedService()
	if err != nil {
		return c.Renderer.Failure(err)
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	request := connect.NewRequest(&protocol.RevokeDeviceRequest{RequestId: requestID, PrincipalId: principal, DeviceId: deviceID})
	response, err := retryAdminCall(callCtx, func() (*connect.Response[protocol.RevokeDeviceResponse], error) {
		return service.RevokeDevice(callCtx, request)
	})
	if err != nil {
		return c.Renderer.Failure(adminCallError(requestID, err))
	}
	if response.Msg == nil || hasUnknownMessage(response.Msg) || !validArtifactID(response.Msg.RevocationId, identitycontract.DeviceRevocationPrefix) {
		return c.Renderer.Failure(errInvalidIdentityResponse)
	}
	view.ResultID = response.Msg.RevocationId
	return c.renderMutation(view)
}

func (c Command) protectedService() (interface {
	IssueAccessGrant(context.Context, *connect.Request[protocol.IssueAccessGrantRequest]) (*connect.Response[protocol.IssueAccessGrantResponse], error)
	RevokeAccessGrant(context.Context, *connect.Request[protocol.RevokeAccessGrantRequest]) (*connect.Response[protocol.RevokeAccessGrantResponse], error)
	ListAccessGrants(context.Context, *connect.Request[protocol.ListAccessGrantsRequest]) (*connect.Response[protocol.ListAccessGrantsResponse], error)
	RevokeDevice(context.Context, *connect.Request[protocol.RevokeDeviceRequest]) (*connect.Response[protocol.RevokeDeviceResponse], error)
	IssueApplicationEnrollmentTicket(context.Context, *connect.Request[protocol.IssueApplicationEnrollmentTicketRequest]) (*connect.Response[protocol.IssueApplicationEnrollmentTicketResponse], error)
}, error) {
	if c.sessions == nil {
		return nil, errAdministrationUnavailable
	}
	service, err := c.sessions.ProtectedIdentityService()
	if err != nil {
		return nil, errAdministrationUnavailable
	}
	return service, nil
}

// retryAdminCall replays one ambiguous failure with the exact same protobuf
// request and idempotency key. Definitive authentication, authorization,
// validation, conflict, capacity, and cancellation failures are never retried.
func retryAdminCall[T any](ctx context.Context, invoke func() (*connect.Response[T], error)) (*connect.Response[T], error) {
	response, err := invoke()
	if err == nil || ctx.Err() != nil || !ambiguousAdminFailure(err) {
		return response, err
	}
	return invoke()
}

func ambiguousAdminFailure(err error) bool {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return false
	}
	switch connectErr.Code() {
	case connect.CodeUnknown, connect.CodeInternal, connect.CodeUnavailable:
		return true
	default:
		return false
	}
}

func adminCallError(requestID string, err error) error {
	message := fmt.Sprintf("administrative request_id %q failed: %v", requestID, err)
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return connect.NewError(connectErr.Code(), errors.New(message))
	}
	return errors.New(message)
}

func (c Command) adminRequestID(value string) (string, error) {
	if value == "" {
		raw := make([]byte, 16)
		if c.entropy == nil {
			return "", errors.New("request ID entropy is unavailable")
		}
		if _, err := io.ReadFull(c.entropy, raw); err != nil {
			return "", errors.New("request ID entropy is unavailable")
		}
		value = "r1_" + base64.RawURLEncoding.EncodeToString(raw)
	}
	if len(value) > 128 {
		return "", errors.New("--request-id must contain 1 to 128 visible ASCII bytes")
	}
	for _, character := range []byte(value) {
		if character < 0x21 || character > 0x7e {
			return "", errors.New("--request-id must contain 1 to 128 visible ASCII bytes")
		}
	}
	return value, nil
}

func (c Command) confirmGrant(view grantView, requestID string, yes bool) error {
	return c.confirmMutation(mutationFromGrant("grant_mutation", view, requestID, ""), yes)
}

func (c Command) confirmMutation(view mutationView, yes bool) error {
	if c.Renderer.JSON {
		return nil
	}
	c.renderMutationDetails(view)
	if yes {
		return nil
	}
	if c.input == nil {
		return errConfirmationRequired
	}
	_, _ = fmt.Fprint(c.Renderer.Err, "Type yes to continue: ")
	line, err := bufio.NewReader(io.LimitReader(c.input, 32)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return errConfirmationRequired
	}
	if strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r") != "yes" {
		return errConfirmationRequired
	}
	return nil
}

func mutationFromGrant(operation string, view grantView, requestID, result string) mutationView {
	return mutationView{Operation: operation, Principal: view.Subject, TargetNode: view.TargetNode, Actions: append([]string(nil), view.Actions...), Scope: view.Scope, ResourceKind: view.ResourceKind, ResourceID: view.ResourceID, ResourceOwner: view.ResourceOwner, NotBefore: view.NotBefore, NotAfter: view.NotAfter, GrantID: view.ID, RequestID: requestID, ResultID: result}
}

func (c Command) renderEnrollment(principal, node, requestID string, bootstrap bool) int {
	value := struct {
		Principal  string `json:"principal"`
		TargetNode string `json:"target_node"`
		Mode       string `json:"mode"`
		RequestID  string `json:"request_id,omitempty"`
	}{Principal: principal, TargetNode: node, Mode: "administrator", RequestID: requestID}
	if bootstrap {
		value.Mode = "bootstrap"
	}
	if c.Renderer.JSON {
		return c.renderJSON(value)
	}
	c.Renderer.Header("Principal enrollment")
	c.Renderer.KV("principal", principal)
	c.Renderer.KV("target_node", node)
	c.Renderer.KV("mode", value.Mode)
	if requestID != "" {
		c.Renderer.KV("request_id", requestID)
	}
	return 0
}

func (c Command) renderGrant(view grantView) {
	c.Renderer.Header("Access Grant")
	c.Renderer.KV("id", view.ID)
	c.Renderer.KV("principal", view.Subject)
	c.Renderer.KV("target_node", view.TargetNode)
	c.Renderer.KV("actions", strings.Join(view.Actions, ","))
	c.Renderer.KV("scope", view.Scope)
	if view.Scope == "exact" {
		c.Renderer.KV("resource_kind", view.ResourceKind)
		c.Renderer.KV("resource_id", view.ResourceID)
		if view.ResourceOwner != "" {
			c.Renderer.KV("resource_owner", view.ResourceOwner)
		}
	}
	c.Renderer.KV("not_before", view.NotBefore)
	c.Renderer.KV("not_after", view.NotAfter)
	c.Renderer.KV("revoked", fmt.Sprint(view.Revoked))
}

func (c Command) renderMutation(view mutationView) int {
	if c.Renderer.JSON {
		return c.renderJSON(view)
	}
	if view.ResultID != "" {
		c.Renderer.KV("result_id", view.ResultID)
	}
	return 0
}

func (c Command) renderMutationDetails(view mutationView) {
	c.Renderer.Header("Identity mutation")
	c.Renderer.KV("operation", view.Operation)
	c.Renderer.KV("principal", view.Principal)
	c.Renderer.KV("target_node", view.TargetNode)
	if len(view.Actions) != 0 {
		c.Renderer.KV("actions", strings.Join(view.Actions, ","))
		c.Renderer.KV("scope", view.Scope)
		if view.Scope == "exact" {
			c.Renderer.KV("resource_kind", view.ResourceKind)
			c.Renderer.KV("resource_id", view.ResourceID)
			if view.ResourceOwner != "" {
				c.Renderer.KV("resource_owner", view.ResourceOwner)
			}
		}
		c.Renderer.KV("not_before", view.NotBefore)
		c.Renderer.KV("not_after", view.NotAfter)
	}
	if view.DeviceID != "" {
		c.Renderer.KV("device_id", view.DeviceID)
	}
	if view.GrantID != "" {
		c.Renderer.KV("grant_id", view.GrantID)
	}
	if view.RequestID != "" {
		c.Renderer.KV("request_id", view.RequestID)
	}
}

func hasUnknownMessage(message proto.Message) bool {
	if message == nil {
		return true
	}
	return hasUnknownReflect(message.ProtoReflect())
}

func hasUnknownReflect(message protoreflect.Message) bool {
	if len(message.GetUnknown()) != 0 {
		return true
	}
	unknown := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsList() && field.Message() != nil {
			list := value.List()
			for index := 0; index < list.Len(); index++ {
				if hasUnknownReflect(list.Get(index).Message()) {
					unknown = true
					return false
				}
			}
		} else if field.IsMap() && field.MapValue().Message() != nil {
			value.Map().Range(func(_ protoreflect.MapKey, item protoreflect.Value) bool {
				unknown = hasUnknownReflect(item.Message())
				return !unknown
			})
		} else if field.Message() != nil && hasUnknownReflect(value.Message()) {
			unknown = true
		}
		return !unknown
	})
	return unknown
}

func validArtifactID(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+52 {
		return false
	}
	suffix := value[len(prefix):]
	for _, character := range suffix {
		if !((character >= 'a' && character <= 'z') || (character >= '2' && character <= '7')) {
			return false
		}
	}
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	raw, err := encoding.DecodeString(strings.ToUpper(suffix))
	nonzero := false
	for _, value := range raw {
		nonzero = nonzero || value != 0
	}
	return err == nil && len(raw) == 32 && nonzero && strings.ToLower(encoding.EncodeToString(raw)) == suffix
}

func visibleASCII(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range []byte(value) {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func grantUsage(writer io.Writer) {
	_, _ = fmt.Fprintln(writer, "Usage: ardentsctl [global flags] identity grant <list|issue|revoke> [flags]")
}

var _ flag.Value = (*stringList)(nil)
