package authority

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	domain "ardents/internal/authority"
	"ardents/internal/cli/client"
	"ardents/internal/cli/output"
	protocol "ardents/internal/localapi/protocol"
)

func (c *Command) membership(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		output.Writeln(
			c.ctx.Renderer.Out,
			"Usage: ardentsctl authority membership <change|fence>",
		)
		return 0
	}
	switch args[0] {
	case "change":
		return c.changeMembership(ctx, args[1:])
	case "fence":
		return c.submitMembershipFence(ctx, args[1:])
	default:
		return c.ctx.Failure(flag.ErrHelp)
	}
}

func (c *Command) changeMembership(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("authority membership change", flag.ContinueOnError)
	fs.SetOutput(c.ctx.Renderer.Err)
	var requestID, realmID, channelHex, change, target, outPath string
	var attestationPaths repeatedPaths
	var validFor, drainFor time.Duration
	fs.StringVar(&requestID, "request-id", "", "stable membership request identity")
	fs.StringVar(&realmID, "realm-id", "", "exact Realm identifier")
	fs.StringVar(&channelHex, "channel-id", "", "32 lowercase hexadecimal characters")
	fs.StringVar(&change, "change", "", "exact mutation: add or remove")
	fs.StringVar(&target, "target-principal", "", "Principal being added or removed")
	fs.Var(&attestationPaths, "attestation-file", "protected next-generation recipient attestation; repeat per recipient")
	fs.DurationVar(&validFor, "valid-for", time.Hour, "new generation validity")
	fs.DurationVar(&drainFor, "drain-for", 15*time.Minute, "previous-generation receive drain")
	fs.StringVar(&outPath, "out-file", "", "new protected membership rotation file")
	if err := fs.Parse(args); err != nil {
		return c.ctx.Failure(err)
	}
	channelRaw, err := hex.DecodeString(channelHex)
	if err != nil || len(channelRaw) != 16 || hex.EncodeToString(channelRaw) != channelHex ||
		requestID == "" || realmID == "" || target == "" ||
		(change != domain.MembershipChangeAdd && change != domain.MembershipChangeRemove) ||
		len(attestationPaths) == 0 ||
		validFor <= 0 || validFor > 30*24*time.Hour || validFor%time.Second != 0 ||
		drainFor <= 0 || drainFor > domain.MaximumPreviousGenerationDrain ||
		drainFor%time.Second != 0 || !filepath.IsAbs(outPath) || fs.NArg() != 0 {
		return c.ctx.Failure(flag.ErrHelp)
	}
	attestations := make([]*protocol.GenerationDeliveryAttestation, 0, len(attestationPaths))
	for _, path := range attestationPaths {
		if !filepath.IsAbs(path) {
			return c.ctx.Failure(flag.ErrHelp)
		}
		attestation := &protocol.GenerationDeliveryAttestation{}
		if err := readProtectedProto(path, attestation); err != nil {
			return c.ctx.Failure(err)
		}
		attestations = append(attestations, attestation)
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	response, err := c.ctx.Client.Service().ChangeChannelMembership(
		callCtx, client.Request(&protocol.ChangeChannelMembershipRequest{
			Version: domain.ContractVersion, RequestId: requestID,
			RealmId: realmID, ChannelId: channelRaw, Change: change,
			TargetPrincipal: target, RecipientAttestations: attestations,
			ValidForSeconds: uint64(validFor / time.Second),
			DrainForSeconds: uint64(drainFor / time.Second),
		}),
	)
	if err != nil {
		return c.ctx.Failure(err)
	}
	if err := writeProtectedProto(outPath, response.Msg); err != nil {
		return c.ctx.Failure(err)
	}
	for _, path := range attestationPaths {
		if err := os.Remove(path); err != nil {
			return c.ctx.Failure(fmt.Errorf("remove consumed attestation: %w", err))
		}
	}
	renderDeliveryMetadata(c, response.Msg.GetStatus(), map[string]string{
		"realm_id":          response.Msg.GetRealmId(),
		"operation_id":      response.Msg.GetOperationId(),
		"membership_change": response.Msg.GetMembershipChange(),
		"target_principal":  response.Msg.GetTargetPrincipal(),
		"member_state":      response.Msg.GetMemberState(),
		"phase":             response.Msg.GetPhase(), "artifact": outPath,
	})
	return c.ctx.Renderer.MutationOutcome(response.Msg.GetStatus())
}

func (c *Command) submitMembershipFence(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("authority membership fence", flag.ContinueOnError)
	fs.SetOutput(c.ctx.Renderer.Err)
	var realmID, channelHex, operationID, evidencePath string
	fs.StringVar(&realmID, "realm-id", "", "exact Realm identifier")
	fs.StringVar(&channelHex, "channel-id", "", "32 lowercase hexadecimal characters")
	fs.StringVar(&operationID, "operation-id", "", "membership operation identity")
	fs.StringVar(&evidencePath, "evidence-file", "", "protected DeploymentFenceEvidence/v1 file")
	if err := fs.Parse(args); err != nil {
		return c.ctx.Failure(err)
	}
	channelRaw, err := hex.DecodeString(channelHex)
	if err != nil || len(channelRaw) != 16 || hex.EncodeToString(channelRaw) != channelHex ||
		realmID == "" || operationID == "" || !filepath.IsAbs(evidencePath) ||
		fs.NArg() != 0 {
		return c.ctx.Failure(flag.ErrHelp)
	}
	evidence := &protocol.DeploymentFenceEvidence{}
	if err := readProtectedProto(evidencePath, evidence); err != nil {
		return c.ctx.Failure(err)
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	response, err := c.ctx.Client.Service().SubmitDeploymentFenceEvidence(
		callCtx, client.Request(&protocol.SubmitDeploymentFenceEvidenceRequest{
			Version: domain.ContractVersion, RealmId: realmID,
			ChannelId: channelRaw, OperationId: operationID, Evidence: evidence,
		}),
	)
	if err != nil {
		return c.ctx.Failure(err)
	}
	if err := os.Remove(evidencePath); err != nil {
		return c.ctx.Failure(fmt.Errorf("remove consumed fence evidence: %w", err))
	}
	renderDeliveryMetadata(c, response.Msg.GetStatus(), map[string]string{
		"realm_id":         response.Msg.GetRealmId(),
		"operation_id":     response.Msg.GetOperationId(),
		"target_principal": response.Msg.GetTargetPrincipal(),
		"evidence_digest":  response.Msg.GetEvidenceDigest(),
		"phase":            response.Msg.GetPhase(),
	})
	return c.ctx.Renderer.MutationOutcome(response.Msg.GetStatus())
}
