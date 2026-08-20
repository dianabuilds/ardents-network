package releasedecision

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

const (
	rootSchemaField  = "ardents_schema_version"
	rootProfileField = "ardents_profile"
	rootEnvField     = "ardents_environment"
	rootNetworkField = "ardents_network"
)

type rootPolicy struct {
	environment string
	network     string
}

func validateRootPolicy(root *metadata.Metadata[metadata.RootType], local LocalEnvironment, refTime time.Time, previous *rootPolicy) (rootPolicy, error) {
	if root == nil {
		return rootPolicy{}, errors.New("trusted root is missing")
	}
	if root.Signed.IsExpired(refTime) {
		if previous != nil {
			return rootPolicy{}, errors.New("trusted root rotation contains an expired candidate")
		}
		return rootPolicy{}, &metadata.ErrExpiredMetadata{Msg: "trusted root is expired"}
	}
	fields := root.Signed.UnrecognizedFields
	if len(fields) != 4 || integerField(fields[rootSchemaField]) != targetSchemaVersion || fields[rootProfileField] != targetProfile {
		return rootPolicy{}, errors.New("trusted root schema or profile is invalid")
	}
	environment, environmentOK := fields[rootEnvField].(string)
	network, networkOK := fields[rootNetworkField].(string)
	if !environmentOK || !networkOK || environment == "" || network == "" {
		return rootPolicy{}, errors.New("trusted root environment binding is invalid")
	}
	if environment != local.Environment || network != local.Network {
		return rootPolicy{}, errors.New("trusted root does not match the local environment")
	}
	if previous != nil && (environment != previous.environment || network != previous.network) {
		return rootPolicy{}, errors.New("trusted root rotation changes environment or network")
	}
	for _, name := range metadata.TOP_LEVEL_ROLE_NAMES {
		role := root.Signed.Roles[name]
		if role == nil || len(role.KeyIDs) != totalTopLevelKeys || role.Threshold != ordinaryThreshold {
			return rootPolicy{}, fmt.Errorf("trusted root role %s has an invalid threshold", name)
		}
		for _, keyID := range role.KeyIDs {
			if root.Signed.Keys[keyID] == nil {
				return rootPolicy{}, fmt.Errorf("trusted root role %s refers to an unknown key", name)
			}
		}
	}
	return rootPolicy{environment: environment, network: network}, nil
}

func integerField(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case float64:
		if number != math.Trunc(number) {
			return 0
		}
		return int(number)
	default:
		return 0
	}
}
