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

func (c *Command) rotation(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		output.Writeln(c.ctx.Renderer.Out, "Usage: ardentsctl authority rotation <rotate|renew|install|acknowledge-installed|commit|activate|acknowledge-active>")
		return 0
	}
	switch args[0] {
	case "rotate":
		return c.rotateChannel(ctx, args[1:])
	case "renew":
		return c.renewChannel(ctx, args[1:])
	case "install":
		return c.installRotation(ctx, args[1:])
	case "acknowledge-installed":
		return c.acknowledgeRotationInstalled(ctx, args[1:])
	case "commit":
		return c.commitRotation(ctx, args[1:])
	case "activate":
		return c.activateRotation(ctx, args[1:])
	case "acknowledge-active":
		return c.acknowledgeRotationActive(ctx, args[1:])
	default:
		return c.ctx.Failure(flag.ErrHelp)
	}
}

func (c *Command) renewChannel(ctx context.Context, args []string) int {
	return c.rotateOrRenewChannel(ctx, args, true)
}

type repeatedPaths []string

func (paths *repeatedPaths) String() string { return fmt.Sprint([]string(*paths)) }
func (paths *repeatedPaths) Set(value string) error {
	*paths = append(*paths, value)
	return nil
}

func (c *Command) rotateChannel(ctx context.Context, args []string) int {
	return c.rotateOrRenewChannel(ctx, args, false)
}

type rotationCommandInput struct {
	requestID, realmID, outPath string
	channelID                   []byte
	attestationPaths            repeatedPaths
	attestations                []*protocol.GenerationDeliveryAttestation
	validFor, drainFor          time.Duration
}

func (c *Command) rotateOrRenewChannel(
	ctx context.Context,
	args []string,
	renewal bool,
) int {
	input, err := c.parseRotationCommandInput(args, renewal)
	if err != nil {
		return c.ctx.Failure(err)
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	var response *protocol.RotateChannelResponse
	if renewal {
		wire, callErr := c.ctx.Client.Service().RenewChannelGrants(
			callCtx, client.Request(&protocol.RenewChannelGrantsRequest{
				Version: domain.ContractVersion, RequestId: input.requestID,
				RealmId: input.realmID, ChannelId: input.channelID,
				RecipientAttestations: input.attestations,
				DrainForSeconds:       uint64(input.drainFor / time.Second),
			}),
		)
		if callErr != nil {
			return c.ctx.Failure(callErr)
		}
		response = wire.Msg
	} else {
		wire, callErr := c.ctx.Client.Service().RotateChannel(
			callCtx, client.Request(&protocol.RotateChannelRequest{
				Version: domain.ContractVersion, RequestId: input.requestID,
				RealmId: input.realmID, ChannelId: input.channelID,
				RecipientAttestations: input.attestations,
				ValidForSeconds:       uint64(input.validFor / time.Second),
				DrainForSeconds:       uint64(input.drainFor / time.Second),
			}),
		)
		if callErr != nil {
			return c.ctx.Failure(callErr)
		}
		response = wire.Msg
	}
	return c.finishRotationCommand(input, response)
}

func (c *Command) parseRotationCommandInput(
	args []string,
	renewal bool,
) (rotationCommandInput, error) {
	name, requestDescription, outputDescription := "authority rotation rotate",
		"stable rotation request identity", "new protected rotation file"
	if renewal {
		name, requestDescription, outputDescription = "authority rotation renew",
			"stable renewal request identity", "new protected renewal file"
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(c.ctx.Renderer.Err)
	input := rotationCommandInput{validFor: time.Hour, drainFor: 15 * time.Minute}
	var channelHex string
	fs.StringVar(&input.requestID, "request-id", "", requestDescription)
	fs.StringVar(&input.realmID, "realm-id", "", "exact Realm identifier")
	fs.StringVar(&channelHex, "channel-id", "", "32 lowercase hexadecimal characters")
	fs.Var(&input.attestationPaths, "attestation-file", "protected member attestation file; repeat per member")
	if !renewal {
		fs.DurationVar(&input.validFor, "valid-for", time.Hour, "new generation validity")
	}
	fs.DurationVar(&input.drainFor, "drain-for", 15*time.Minute, "previous-generation receive drain")
	fs.StringVar(&input.outPath, "out-file", "", outputDescription)
	if err := fs.Parse(args); err != nil {
		return rotationCommandInput{}, err
	}
	channelRaw, err := hex.DecodeString(channelHex)
	if err != nil || len(channelRaw) != 16 || hex.EncodeToString(channelRaw) != channelHex ||
		input.requestID == "" || input.realmID == "" || len(input.attestationPaths) == 0 ||
		len(input.attestationPaths) > domain.MaxMembersPerChannel ||
		input.validFor <= 0 || input.validFor > 30*24*time.Hour ||
		input.validFor%time.Second != 0 ||
		input.drainFor <= 0 || input.drainFor > domain.MaximumPreviousGenerationDrain ||
		input.drainFor%time.Second != 0 || !filepath.IsAbs(input.outPath) ||
		fs.NArg() != 0 {
		return rotationCommandInput{}, flag.ErrHelp
	}
	input.channelID = channelRaw
	input.attestations = make(
		[]*protocol.GenerationDeliveryAttestation, 0, len(input.attestationPaths),
	)
	for _, path := range input.attestationPaths {
		if !filepath.IsAbs(path) {
			return rotationCommandInput{}, flag.ErrHelp
		}
		attestation := &protocol.GenerationDeliveryAttestation{}
		if err := readProtectedProto(path, attestation); err != nil {
			return rotationCommandInput{}, err
		}
		input.attestations = append(input.attestations, attestation)
	}
	return input, nil
}

func (c *Command) finishRotationCommand(
	input rotationCommandInput,
	response *protocol.RotateChannelResponse,
) int {
	if err := writeProtectedProto(input.outPath, response); err != nil {
		return c.ctx.Failure(err)
	}
	for _, path := range input.attestationPaths {
		if err := os.Remove(path); err != nil {
			return c.ctx.Failure(fmt.Errorf("remove consumed attestation: %w", err))
		}
	}
	renderDeliveryMetadata(c, response.GetStatus(), map[string]string{
		"realm_id": response.GetRealmId(), "operation_id": response.GetOperationId(),
		"phase": response.GetPhase(), "artifact": input.outPath,
	})
	return c.ctx.Renderer.MutationOutcome(response.GetStatus())
}

func (c *Command) installRotation(ctx context.Context, args []string) int {
	fs, rotationPath := c.rotationArtifactFlagSet("authority rotation install")
	var receiptPath, recipient string
	fs.StringVar(&recipient, "recipient", "", "local member Principal when the rotation has multiple deliveries")
	fs.StringVar(&receiptPath, "out-file", "", "new protected installed receipt file")
	if err := fs.Parse(args); err != nil {
		return c.ctx.Failure(err)
	}
	if !filepath.IsAbs(receiptPath) || fs.NArg() != 0 {
		return c.ctx.Failure(flag.ErrHelp)
	}
	rotation, code := c.loadRotationArtifact(*rotationPath)
	if code >= 0 {
		return code
	}
	var sealed *protocol.SealedGenerationDelivery
	switch {
	case recipient == "" && len(rotation.GetDeliveries()) == 1:
		sealed = rotation.GetDeliveries()[0].GetSealed()
	case recipient != "":
		for _, delivery := range rotation.GetDeliveries() {
			if delivery.GetRecipientPrincipal() == recipient {
				sealed = delivery.GetSealed()
				break
			}
		}
	}
	if sealed == nil {
		return c.ctx.Failure(fmt.Errorf("rotation has no unique delivery for the requested member"))
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	response, err := c.ctx.Client.Service().InstallGenerationDelivery(
		callCtx, client.Request(&protocol.InstallGenerationDeliveryRequest{
			Version: domain.ContractVersion, Sealed: sealed,
		}),
	)
	if err != nil {
		return c.ctx.Failure(err)
	}
	if err := writeProtectedProto(receiptPath, response.Msg.GetReceipt()); err != nil {
		return c.ctx.Failure(err)
	}
	renderDeliveryMetadata(c, response.Msg.GetStatus(), map[string]string{
		"delivery_id": response.Msg.GetReceipt().GetDeliveryId(), "artifact": receiptPath,
		"phase": "installed", "operation_id": rotation.GetOperationId(), "realm_id": rotation.GetRealmId(),
	})
	return c.ctx.Renderer.MutationOutcome(response.Msg.GetStatus())
}

func (c *Command) acknowledgeRotationInstalled(ctx context.Context, args []string) int {
	rotation, _, receipt, receiptPath, code := c.rotationAndReceiptArgs(
		"authority rotation acknowledge-installed", args,
	)
	if code >= 0 {
		return code
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	response, err := c.ctx.Client.Service().AcknowledgeInitialGeneration(
		callCtx, client.Request(&protocol.AcknowledgeInitialGenerationRequest{
			Version: domain.ContractVersion, RealmId: rotation.GetRealmId(),
			OperationId: rotation.GetOperationId(), Receipt: receipt,
		}),
	)
	if err != nil {
		return c.ctx.Failure(err)
	}
	if err := os.Remove(receiptPath); err != nil {
		return c.ctx.Failure(fmt.Errorf("remove consumed receipt: %w", err))
	}
	renderDeliveryMetadata(c, response.Msg.GetStatus(), map[string]string{
		"realm_id": rotation.GetRealmId(), "operation_id": rotation.GetOperationId(),
		"delivery_id": receipt.GetDeliveryId(), "phase": response.Msg.GetPhase(),
	})
	return c.ctx.Renderer.MutationOutcome(response.Msg.GetStatus())
}

func (c *Command) commitRotation(ctx context.Context, args []string) int {
	rotation, _, outPath, code := c.rotationAndOutputArgs("authority rotation commit", args)
	if code >= 0 {
		return code
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	response, err := c.ctx.Client.Service().CommitChannelActivation(
		callCtx, client.Request(&protocol.CommitChannelActivationRequest{
			Version: domain.ContractVersion, RealmId: rotation.GetRealmId(),
			OperationId: rotation.GetOperationId(),
		}),
	)
	if err != nil {
		return c.ctx.Failure(err)
	}
	if err := writeProtectedProto(outPath, response.Msg); err != nil {
		return c.ctx.Failure(err)
	}
	renderDeliveryMetadata(c, response.Msg.GetStatus(), map[string]string{
		"realm_id": rotation.GetRealmId(), "operation_id": rotation.GetOperationId(),
		"phase": response.Msg.GetPhase(), "artifact": outPath,
	})
	return c.ctx.Renderer.MutationOutcome(response.Msg.GetStatus())
}

func (c *Command) activateRotation(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("authority rotation activate", flag.ContinueOnError)
	fs.SetOutput(c.ctx.Renderer.Err)
	var activationPath, outPath string
	fs.StringVar(&activationPath, "activation-file", "", "protected committed activation file")
	fs.StringVar(&outPath, "out-file", "", "new protected active receipt file")
	if err := fs.Parse(args); err != nil {
		return c.ctx.Failure(err)
	}
	if !filepath.IsAbs(activationPath) || !filepath.IsAbs(outPath) || fs.NArg() != 0 {
		return c.ctx.Failure(flag.ErrHelp)
	}
	artifact := &protocol.CommitChannelActivationResponse{}
	if err := readProtectedProto(activationPath, artifact); err != nil {
		return c.ctx.Failure(err)
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	response, err := c.ctx.Client.Service().ActivateGeneration(
		callCtx, client.Request(&protocol.ActivateGenerationRequest{
			Version: domain.ContractVersion, Activation: artifact.GetActivation(),
		}),
	)
	if err != nil {
		return c.ctx.Failure(err)
	}
	if err := writeProtectedProto(outPath, response.Msg.GetReceipt()); err != nil {
		return c.ctx.Failure(err)
	}
	renderDeliveryMetadata(c, response.Msg.GetStatus(), map[string]string{
		"operation_id": artifact.GetOperationId(), "delivery_id": response.Msg.GetReceipt().GetDeliveryId(),
		"phase": "active", "artifact": outPath,
	})
	return c.ctx.Renderer.MutationOutcome(response.Msg.GetStatus())
}

func (c *Command) acknowledgeRotationActive(ctx context.Context, args []string) int {
	fs, rotationPath := c.rotationArtifactFlagSet("authority rotation acknowledge-active")
	var receiptPath, disposition string
	fs.StringVar(&receiptPath, "receipt-file", "", "protected member receipt file")
	fs.StringVar(&disposition, "host-disposition", "", "deployment-owned disposition; must be approved")
	if err := fs.Parse(args); err != nil {
		return c.ctx.Failure(err)
	}
	if disposition != "approved" || !filepath.IsAbs(receiptPath) || fs.NArg() != 0 {
		return c.ctx.Failure(flag.ErrHelp)
	}
	rotation, code := c.loadRotationArtifact(*rotationPath)
	if code >= 0 {
		return code
	}
	receipt := &protocol.GenerationDeliveryReceipt{}
	if err := readProtectedProto(receiptPath, receipt); err != nil {
		return c.ctx.Failure(err)
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	response, err := c.ctx.Client.Service().AcknowledgeChannelActivation(
		callCtx, client.Request(&protocol.AcknowledgeChannelActivationRequest{
			Version: domain.ContractVersion, RealmId: rotation.GetRealmId(),
			OperationId: rotation.GetOperationId(), ApprovedHost: true, Receipt: receipt,
		}),
	)
	if err != nil {
		return c.ctx.Failure(err)
	}
	if err := os.Remove(receiptPath); err != nil {
		return c.ctx.Failure(fmt.Errorf("remove consumed receipt: %w", err))
	}
	renderDeliveryMetadata(c, response.Msg.GetStatus(), map[string]string{
		"realm_id": rotation.GetRealmId(), "operation_id": rotation.GetOperationId(),
		"delivery_id": receipt.GetDeliveryId(), "phase": response.Msg.GetPhase(),
	})
	return c.ctx.Renderer.MutationOutcome(response.Msg.GetStatus())
}

func (c *Command) rotationAndOutputArgs(name string, args []string) (*protocol.RotateChannelResponse, string, string, int) {
	fs, rotationPath := c.rotationArtifactFlagSet(name)
	var outPath string
	fs.StringVar(&outPath, "out-file", "", "new protected output file")
	if err := fs.Parse(args); err != nil {
		return nil, "", "", c.ctx.Failure(err)
	}
	if !filepath.IsAbs(outPath) || fs.NArg() != 0 {
		return nil, "", "", c.ctx.Failure(flag.ErrHelp)
	}
	rotation, code := c.loadRotationArtifact(*rotationPath)
	if code >= 0 {
		return nil, "", "", code
	}
	return rotation, *rotationPath, outPath, -1
}

func (c *Command) rotationAndReceiptArgs(name string, args []string) (*protocol.RotateChannelResponse, string, *protocol.GenerationDeliveryReceipt, string, int) {
	fs, rotationPath := c.rotationArtifactFlagSet(name)
	var receiptPath string
	fs.StringVar(&receiptPath, "receipt-file", "", "protected member receipt file")
	if err := fs.Parse(args); err != nil {
		return nil, "", nil, "", c.ctx.Failure(err)
	}
	if !filepath.IsAbs(receiptPath) || fs.NArg() != 0 {
		return nil, "", nil, "", c.ctx.Failure(flag.ErrHelp)
	}
	rotation, code := c.loadRotationArtifact(*rotationPath)
	if code >= 0 {
		return nil, "", nil, "", code
	}
	receipt := &protocol.GenerationDeliveryReceipt{}
	if err := readProtectedProto(receiptPath, receipt); err != nil {
		return nil, "", nil, "", c.ctx.Failure(err)
	}
	return rotation, *rotationPath, receipt, receiptPath, -1
}

func (c *Command) rotationArtifactFlagSet(name string) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(c.ctx.Renderer.Err)
	var rotationPath string
	fs.StringVar(&rotationPath, "rotation-file", "", "protected rotation file")
	return fs, &rotationPath
}

func (c *Command) loadRotationArtifact(path string) (*protocol.RotateChannelResponse, int) {
	if !filepath.IsAbs(path) {
		return nil, c.ctx.Failure(flag.ErrHelp)
	}
	rotation := &protocol.RotateChannelResponse{}
	if err := readProtectedProto(path, rotation); err != nil {
		return nil, c.ctx.Failure(err)
	}
	return rotation, -1
}
