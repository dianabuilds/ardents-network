package namespace

import (
	"reflect"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

func validControlOperation(value controlOperation) bool {
	name, err := naming.Parse(value.Name)
	if err != nil || string(name) != value.Name || value.OperationDigest == [32]byte{} ||
		len(value.OrderingProof) > 2<<10 || len(value.AuthorityProof) > 2<<10 ||
		len(value.RecoveryPolicy) > 2<<10 || len(value.RecoveryProof) > 2<<10 {
		return false
	}
	expected := controlOperation{Kind: value.Kind, OperationDigest: value.OperationDigest, Name: value.Name}
	switch value.Kind {
	case "claim":
		if value.Generation == 0 || value.Authority == [32]byte{} || value.LeaseNotAfter <= 0 ||
			len(value.OrderingProof) == 0 {
			return false
		}
		expected.Generation, expected.Authority = value.Generation, value.Authority
		expected.LeaseNotAfter, expected.OrderingProof = value.LeaseNotAfter, value.OrderingProof
	case "renew":
		if !validExistingControl(value) || value.LeaseNotAfter <= 0 || len(value.AuthorityProof) == 0 {
			return false
		}
		expected.Generation, expected.ExpectedRevision = value.Generation, value.ExpectedRevision
		expected.LeaseNotAfter, expected.AuthorityProof = value.LeaseNotAfter, value.AuthorityProof
	case "record":
		if !validExistingControl(value) || value.Target == [32]byte{} || value.RecordNotAfter <= 0 ||
			len(value.AuthorityProof) == 0 {
			return false
		}
		expected.Generation, expected.ExpectedRevision = value.Generation, value.ExpectedRevision
		expected.Target, expected.RecordNotAfter = value.Target, value.RecordNotAfter
		expected.AuthorityProof = value.AuthorityProof
	case "release":
		if !validExistingControl(value) || len(value.AuthorityProof) == 0 {
			return false
		}
		expected.Generation, expected.ExpectedRevision = value.Generation, value.ExpectedRevision
		expected.AuthorityProof = value.AuthorityProof
	case "transfer":
		if !validExistingControl(value) || value.SuccessorAuthority == [32]byte{} || len(value.AuthorityProof) == 0 {
			return false
		}
		expected.Generation, expected.ExpectedRevision = value.Generation, value.ExpectedRevision
		expected.SuccessorAuthority, expected.AuthorityProof = value.SuccessorAuthority, value.AuthorityProof
	case "delegate":
		parent, parentErr := naming.Parse(value.ParentName)
		if parentErr != nil || string(parent) != value.ParentName || value.ParentGeneration == 0 ||
			value.ParentRevision == 0 || value.ChildGeneration == 0 || value.Authority == [32]byte{} ||
			value.LeaseNotAfter <= 0 || len(value.AuthorityProof) == 0 {
			return false
		}
		expected.ParentName, expected.ParentGeneration = value.ParentName, value.ParentGeneration
		expected.ParentRevision, expected.ChildGeneration = value.ParentRevision, value.ChildGeneration
		expected.Authority, expected.LeaseNotAfter = value.Authority, value.LeaseNotAfter
		expected.AuthorityProof = value.AuthorityProof
	case "policy":
		if !validExistingControl(value) || value.PolicyNotBefore <= 0 || len(value.AuthorityProof) == 0 {
			return false
		}
		expected.Generation, expected.ExpectedRevision = value.Generation, value.ExpectedRevision
		expected.PolicyNotBefore, expected.RecoveryPolicy = value.PolicyNotBefore, value.RecoveryPolicy
		expected.AuthorityProof = value.AuthorityProof
	case "recovery":
		if !validExistingControl(value) || value.PolicyID == [32]byte{} || value.RecoveryNotBefore <= 0 {
			return false
		}
		expected.Generation, expected.ExpectedRevision = value.Generation, value.ExpectedRevision
		expected.PolicyID, expected.RecoveryStep = value.PolicyID, value.RecoveryStep
		expected.RecoveryNotBefore = value.RecoveryNotBefore
		switch value.RecoveryStep {
		case "initiate", "cancel", "complete":
			if len(value.RecoveryProof) == 0 {
				return false
			}
			expected.RecoveryProof = value.RecoveryProof
		case "resume":
			if value.Target == [32]byte{} || value.RecordNotAfter <= 0 || len(value.AuthorityProof) == 0 {
				return false
			}
			expected.Target, expected.RecordNotAfter, expected.AuthorityProof = value.Target, value.RecordNotAfter, value.AuthorityProof
		default:
			return false
		}
	default:
		return false
	}
	return reflect.DeepEqual(value, expected)
}

func validExistingControl(value controlOperation) bool {
	return value.Generation > 0 && value.ExpectedRevision > 0
}
