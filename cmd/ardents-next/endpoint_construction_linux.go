//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
)

// The bounded operator declaration carries local storage roots, sockets,
// execution principals and the two permissions. It owns no network trust
// facts and no plan schema; the constructor authenticates those separately.
type endpointPlanDeclaration struct {
	Role                    string                      `json:"role"`
	NetworkStateRoot        string                      `json:"network_state_root"`
	LocalRoleStateRoot      string                      `json:"local_role_state_root"`
	TextTokenRoot           string                      `json:"text_token_root"`
	EntryStateRoot          string                      `json:"entry_state_root"`
	PublicationRoot         string                      `json:"publication_root"`
	ServiceInstanceRoot     string                      `json:"service_instance_root"`
	TimeConfidenceFile      string                      `json:"time_confidence_file"`
	ApplicationSocket       string                      `json:"application_socket"`
	AdministrationSocket    string                      `json:"administration_socket"`
	BrokerID                string                      `json:"broker_id"`
	ConnectionPrincipal     string                      `json:"connection_principal"`
	AdministrationPrincipal string                      `json:"administration_principal"`
	ReaderPermission        endpointPermissionPlanInput `json:"reader_permission"`
	PublisherPermission     endpointPermissionPlanInput `json:"publisher_permission"`
}

type endpointPermissionPlanInput struct {
	RequestPath  string    `json:"request_path"`
	ResponsePath string    `json:"response_path"`
	Maxima       [3]uint32 `json:"maxima"`
}

// runEndpointPlanConstruction is the production plan constructor: it renders
// headless.json and source.json from authenticated accepted-State facts, the
// operator's bounded local declaration and the distributed Source plan
// artifact. It never edits or adopts existing plan bytes.
func runEndpointPlanConstruction(ctx context.Context, args []string, diagnostic io.Writer) int {
	if ctx == nil || len(args) != 4 {
		return 2
	}
	for _, path := range args {
		if !absoluteAdmissionPath(path) {
			return 2
		}
	}
	if err := ctx.Err(); err != nil {
		return 1
	}
	ownerRaw, err := readBounded(args[0], 8<<10)
	if err != nil {
		return 1
	}
	var owner networkAuthorityPlan
	if decodeAdmissionObject(ownerRaw, &owner) != nil {
		return 1
	}
	// The established local Network owner projection builds the reference
	// configuration; the constructor itself refuses pins that do not open the
	// accepted retained history.
	config, err := networkStateConfig(&owner)
	if err != nil {
		return 1
	}
	sourceRaw, err := readBounded(args[1], 32<<10)
	if err != nil {
		return 1
	}
	declared, err := constructSourceDeclaration(sourceRaw)
	if err != nil {
		return 1
	}
	declarationRaw, err := readBounded(args[2], 8<<10)
	if err != nil {
		return 1
	}
	var input endpointPlanDeclaration
	if decodeAdmissionObject(declarationRaw, &input) != nil {
		return 1
	}
	declaration := endpoint.PlanDeclaration{Role: input.Role, NetworkStateRoot: input.NetworkStateRoot,
		LocalRoleStateRoot: input.LocalRoleStateRoot, TextTokenRoot: input.TextTokenRoot, EntryStateRoot: input.EntryStateRoot,
		PublicationRoot: input.PublicationRoot, ServiceInstanceRoot: input.ServiceInstanceRoot,
		TimeConfidenceFile: input.TimeConfidenceFile, ApplicationSocket: input.ApplicationSocket,
		AdministrationSocket: input.AdministrationSocket, BrokerID: input.BrokerID, ConnectionPrincipal: input.ConnectionPrincipal,
		AdministrationPrincipal: input.AdministrationPrincipal, ReaderPermission: endpoint.PermissionPlan(input.ReaderPermission),
		PublisherPermission: endpoint.PermissionPlan(input.PublisherPermission)}
	_, headlessJSON, sourceJSON, err := endpoint.ConstructPlans(ctx, config, declaration, declared)
	if err != nil {
		return 1
	}
	if err := persistConstructedPlans(ctx, args[3], headlessJSON, sourceJSON); err != nil {
		return 1
	}
	if json.NewEncoder(diagnostic).Encode(map[string]string{"phase": "plan-construction", "outcome": "constructed"}) != nil {
		return 1
	}
	return 0
}

// constructSourceDeclaration decodes the distributed Source plan artifact —
// the grammar `source prepare-trust` emits — with exact fields, its fixed
// schema, exactly two complete members and the established bounded refresh
// grammar.
func constructSourceDeclaration(raw []byte) (endpoint.SourcePlan, error) {
	var declared endpoint.SourcePlan
	if decodeAdmissionObject(raw, &declared) != nil || declared.Schema != "ardents-source-plan-v1" ||
		len(declared.Sources) != 2 || declared.OrderSeed == "" || declared.ClockObservedAt == "" ||
		declared.ClientCertificate == "" || declared.ClientKey == "" ||
		declared.RefreshIntervalMS < 100 || declared.RefreshIntervalMS > 60000 {
		return endpoint.SourcePlan{}, errors.New("source plan declaration is unavailable")
	}
	for _, member := range declared.Sources {
		if member.Address == "" || member.ServerName == "" || member.Identity == "" || member.Family == "" ||
			member.EndpointHandle == "" || member.RootCA == "" || member.LeafKeyDigest == "" {
			return endpoint.SourcePlan{}, errors.New("source plan declaration is unavailable")
		}
	}
	return declared, nil
}

// persistConstructedPlans durably renders both plan files. Existing bytes are
// neither adopted nor overwritten: an interrupted construction is retried
// into a fresh directory, never repaired in place.
func persistConstructedPlans(ctx context.Context, directory string, headless, source []byte) (result error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if info, err := os.Lstat(directory); err != nil || !info.IsDir() {
		return errors.Join(errors.New("construction output directory is unavailable"), err)
	}
	files := []struct {
		name string
		body []byte
	}{{"headless.json", headless}, {"source.json", source}}
	for _, file := range files {
		if _, err := os.Lstat(filepath.Join(directory, file.name)); err == nil {
			return errors.New("constructed plan output already exists")
		}
	}
	var opened []*os.File
	defer func() {
		for i := len(opened) - 1; i >= 0; i-- {
			result = errors.Join(result, opened[i].Close())
		}
		result = errors.Join(result, ctx.Err())
	}()
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		handle, err := os.OpenFile(filepath.Join(directory, file.name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		opened = append(opened, handle)
		if _, err := handle.Write(file.body); err != nil {
			return err
		}
		if err := handle.Sync(); err != nil {
			return err
		}
	}
	directoryFile, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, directoryFile.Close()) }()
	return directoryFile.Sync()
}
