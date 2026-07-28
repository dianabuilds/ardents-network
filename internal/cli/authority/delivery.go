package authority

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	domain "ardents/internal/authority"
	"ardents/internal/cli/client"
	"ardents/internal/cli/output"
	identityapi "ardents/internal/identity"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/storage"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const maximumDeliveryArtifactBytes = int64(512 << 10)

func (c *Command) delivery(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		output.Writeln(c.ctx.Renderer.Out, "Usage: ardentsctl authority delivery <prepare|issue|install|acknowledge>")
		return 0
	}
	switch args[0] {
	case "prepare":
		return c.prepareDelivery(ctx, args[1:])
	case "issue":
		return c.issueDelivery(ctx, args[1:])
	case "install":
		return c.installDelivery(ctx, args[1:])
	case "acknowledge":
		return c.acknowledgeDelivery(ctx, args[1:])
	default:
		return c.ctx.Failure(flag.ErrHelp)
	}
}

func (c *Command) prepareDelivery(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("authority delivery prepare", flag.ContinueOnError)
	fs.SetOutput(c.ctx.Renderer.Err)
	var subject, outPath string
	var validFor time.Duration
	fs.StringVar(&subject, "subject", "", "local member Principal")
	fs.DurationVar(&validFor, "valid-for", time.Hour, "finite attestation validity")
	fs.StringVar(&outPath, "out-file", "", "new protected attestation file")
	if err := fs.Parse(args); err != nil {
		return c.ctx.Failure(err)
	}
	if subject == "" || validFor <= 0 || validFor > 30*24*time.Hour ||
		validFor%time.Second != 0 || !filepath.IsAbs(outPath) || fs.NArg() != 0 {
		return c.ctx.Failure(flag.ErrHelp)
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	response, err := c.ctx.Client.Service().PrepareGenerationDelivery(
		callCtx, client.Request(&protocol.PrepareGenerationDeliveryRequest{
			Version: 1, SubjectPrincipal: subject,
			ValidForSeconds: uint64(validFor / time.Second),
		}),
	)
	if err != nil {
		return c.ctx.Failure(err)
	}
	if err := writeProtectedProto(outPath, response.Msg.GetAttestation()); err != nil {
		return c.ctx.Failure(err)
	}
	renderDeliveryMetadata(c, response.Msg.GetStatus(), map[string]string{
		"subject_principal": subject, "artifact": outPath,
	})
	return c.ctx.Renderer.MutationOutcome(response.Msg.GetStatus())
}

func (c *Command) issueDelivery(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("authority delivery issue", flag.ContinueOnError)
	fs.SetOutput(c.ctx.Renderer.Err)
	var requestID, realmID, channelClass, attestationPath, outPath string
	var permissions uint
	var validFor time.Duration
	fs.StringVar(&requestID, "request-id", "", "stable delivery request identity")
	fs.StringVar(&realmID, "realm-id", "", "exact Realm identifier")
	fs.StringVar(&channelClass, "channel-class", "", "channel capability class")
	fs.UintVar(&permissions, "permissions", 0, "capability permission bitset")
	fs.DurationVar(&validFor, "valid-for", time.Hour, "grant validity")
	fs.StringVar(&attestationPath, "attestation-file", "", "protected attestation file")
	fs.StringVar(&outPath, "out-file", "", "new protected sealed-delivery file")
	if err := fs.Parse(args); err != nil {
		return c.ctx.Failure(err)
	}
	if requestID == "" || realmID == "" || channelClass == "" || permissions == 0 ||
		identityapi.CapabilityPermission(permissions)&^identityapi.CapabilityKnownPermissions != 0 ||
		validFor <= 0 || validFor > 30*24*time.Hour || validFor%time.Second != 0 ||
		!filepath.IsAbs(attestationPath) || !filepath.IsAbs(outPath) || fs.NArg() != 0 {
		return c.ctx.Failure(flag.ErrHelp)
	}
	attestation := &protocol.GenerationDeliveryAttestation{}
	if err := readProtectedProto(attestationPath, attestation); err != nil {
		return c.ctx.Failure(err)
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	response, err := c.ctx.Client.Service().IssueInitialGeneration(
		callCtx, client.Request(&protocol.IssueInitialGenerationRequest{
			Version: domain.ContractVersion, RequestId: requestID, RealmId: realmID,
			ChannelClass: channelClass, Permissions: uint32(permissions),
			RecipientAttestation: attestation, ValidForSeconds: uint64(validFor / time.Second),
		}),
	)
	if err != nil {
		return c.ctx.Failure(err)
	}
	if err := writeProtectedProto(outPath, response.Msg); err != nil {
		return c.ctx.Failure(err)
	}
	if err := os.Remove(attestationPath); err != nil {
		return c.ctx.Failure(fmt.Errorf("remove consumed attestation: %w", err))
	}
	renderDeliveryMetadata(c, response.Msg.GetStatus(), map[string]string{
		"realm_id": response.Msg.GetRealmId(), "operation_id": response.Msg.GetOperationId(),
		"delivery_id": response.Msg.GetDeliveryId(), "artifact": outPath,
	})
	return c.ctx.Renderer.MutationOutcome(response.Msg.GetStatus())
}

func (c *Command) installDelivery(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("authority delivery install", flag.ContinueOnError)
	fs.SetOutput(c.ctx.Renderer.Err)
	var deliveryPath, outPath string
	fs.StringVar(&deliveryPath, "delivery-file", "", "protected sealed-delivery file")
	fs.StringVar(&outPath, "out-file", "", "new protected receipt file")
	if err := fs.Parse(args); err != nil {
		return c.ctx.Failure(err)
	}
	if !filepath.IsAbs(deliveryPath) || !filepath.IsAbs(outPath) || fs.NArg() != 0 {
		return c.ctx.Failure(flag.ErrHelp)
	}
	delivery := &protocol.IssueInitialGenerationResponse{}
	if err := readProtectedProto(deliveryPath, delivery); err != nil {
		return c.ctx.Failure(err)
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	response, err := c.ctx.Client.Service().InstallGenerationDelivery(
		callCtx, client.Request(&protocol.InstallGenerationDeliveryRequest{
			Version: domain.ContractVersion, Sealed: delivery.GetSealed(),
		}),
	)
	if err != nil {
		return c.ctx.Failure(err)
	}
	if err := writeProtectedProto(outPath, response.Msg.GetReceipt()); err != nil {
		return c.ctx.Failure(err)
	}
	renderDeliveryMetadata(c, response.Msg.GetStatus(), map[string]string{
		"delivery_id": response.Msg.GetReceipt().GetDeliveryId(), "artifact": outPath,
	})
	return c.ctx.Renderer.MutationOutcome(response.Msg.GetStatus())
}

func (c *Command) acknowledgeDelivery(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("authority delivery acknowledge", flag.ContinueOnError)
	fs.SetOutput(c.ctx.Renderer.Err)
	var deliveryPath, receiptPath string
	fs.StringVar(&deliveryPath, "delivery-file", "", "protected sealed-delivery file")
	fs.StringVar(&receiptPath, "receipt-file", "", "protected installed receipt file")
	if err := fs.Parse(args); err != nil {
		return c.ctx.Failure(err)
	}
	if !filepath.IsAbs(deliveryPath) || !filepath.IsAbs(receiptPath) || fs.NArg() != 0 {
		return c.ctx.Failure(flag.ErrHelp)
	}
	delivery := &protocol.IssueInitialGenerationResponse{}
	receipt := &protocol.GenerationDeliveryReceipt{}
	if err := readProtectedProto(deliveryPath, delivery); err != nil {
		return c.ctx.Failure(err)
	}
	if err := readProtectedProto(receiptPath, receipt); err != nil {
		return c.ctx.Failure(err)
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	response, err := c.ctx.Client.Service().AcknowledgeInitialGeneration(
		callCtx, client.Request(&protocol.AcknowledgeInitialGenerationRequest{
			Version: domain.ContractVersion, RealmId: delivery.GetRealmId(),
			OperationId: delivery.GetOperationId(), Receipt: receipt,
		}),
	)
	if err != nil {
		return c.ctx.Failure(err)
	}
	if err := os.Remove(receiptPath); err != nil {
		return c.ctx.Failure(fmt.Errorf("remove consumed receipt: %w", err))
	}
	if err := os.Remove(deliveryPath); err != nil {
		return c.ctx.Failure(fmt.Errorf("remove acknowledged delivery: %w", err))
	}
	renderDeliveryMetadata(c, response.Msg.GetStatus(), map[string]string{
		"realm_id": response.Msg.GetRealmId(), "delivery_id": response.Msg.GetDeliveryId(),
		"phase": response.Msg.GetPhase(),
	})
	return c.ctx.Renderer.MutationOutcome(response.Msg.GetStatus())
}

func writeProtectedProto(path string, message proto.Message) error {
	if message == nil {
		return fmt.Errorf("delivery artifact is missing")
	}
	raw, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(message)
	if err != nil {
		return err
	}
	defer clear(raw)
	if int64(len(raw)) > maximumDeliveryArtifactBytes {
		return fmt.Errorf("delivery artifact exceeds size limit")
	}
	return storage.AtomicCreatePrivateFile(path, raw)
}

func readProtectedProto(path string, message proto.Message) error {
	raw, found, err := storage.ReadStrictPrivateFileBounded(path, maximumDeliveryArtifactBytes)
	if err != nil || !found {
		return fmt.Errorf("protected delivery artifact is unavailable")
	}
	defer clear(raw)
	return protojson.UnmarshalOptions{DiscardUnknown: false}.Unmarshal(raw, message)
}

func renderDeliveryMetadata(c *Command, status *protocol.OperationStatus, values map[string]string) {
	if c.ctx.Renderer.JSON {
		document := map[string]any{"status": status.GetState(), "accepted": status.GetAccepted()}
		for key, value := range values {
			document[key] = value
		}
		raw, _ := json.Marshal(document)
		output.Writeln(c.ctx.Renderer.Out, string(raw))
		return
	}
	output.Header(c.ctx.Renderer.Out, "authority delivery")
	output.Status(c.ctx.Renderer.Out, status)
	for _, key := range []string{"realm_id", "operation_id", "delivery_id", "phase", "subject_principal", "artifact"} {
		output.KV(c.ctx.Renderer.Out, key, values[key])
	}
}
