package identity

import "ardents/internal/localapi/protocol/ardentsv1connect"

type accessClass uint8

const (
	accessPublicBounded accessClass = iota + 1
	accessSessionLifecycle
	accessProtected
)

type procedureRule struct {
	class        accessClass
	action       string
	resourceKind string
	mutating     bool
}

var procedureCatalog = map[string]procedureRule{
	ardentsv1connect.IdentityServiceBeginAuthenticationProcedure:              {class: accessPublicBounded},
	ardentsv1connect.IdentityServiceCompleteAuthenticationProcedure:           {class: accessPublicBounded, mutating: true},
	ardentsv1connect.IdentityServiceEndSessionProcedure:                       {class: accessSessionLifecycle, mutating: true},
	ardentsv1connect.IdentityServiceEnrollFirstPrincipalProcedure:             {class: accessPublicBounded, mutating: true},
	ardentsv1connect.IdentityServiceEnrollPrincipalProcedure:                  protectedIdentityRule("identity.principal.enroll", "principal", true),
	ardentsv1connect.IdentityServiceRevokeDeviceProcedure:                     protectedIdentityRule("identity.device.revoke", "device", true),
	ardentsv1connect.IdentityServiceListDeviceRevocationsProcedure:            protectedIdentityRule("identity.device-revocations.list", "device-revocation-collection", false),
	ardentsv1connect.IdentityServiceIssueAccessGrantProcedure:                 protectedIdentityRule("identity.grant.issue", "grant-proposal", true),
	ardentsv1connect.IdentityServiceRevokeAccessGrantProcedure:                protectedIdentityRule("identity.grant.revoke", "access-grant", true),
	ardentsv1connect.IdentityServiceListAccessGrantsProcedure:                 protectedIdentityRule("identity.grant.list", "grant-collection", false),
	ardentsv1connect.IdentityServiceIssueApplicationEnrollmentTicketProcedure: protectedIdentityRule("identity.principal.enroll", "principal", true),
	ardentsv1connect.IdentityServiceImportDelegationRevocationProcedure:       {class: accessPublicBounded, mutating: true},
}

func protectedIdentityRule(action, resourceKind string, mutating bool) procedureRule {
	return procedureRule{class: accessProtected, action: action, resourceKind: resourceKind, mutating: mutating}
}

type RuleClass string

const (
	RuleClassPublicBounded    RuleClass = "public-bounded"
	RuleClassSessionLifecycle RuleClass = "session-lifecycle"
	RuleClassProtected        RuleClass = "protected"
)

type Rule struct {
	Class        RuleClass
	Action       string
	ResourceKind string
	Mutating     bool
}

func RuleForProcedure(procedure string) (Rule, bool) {
	rule, ok := procedureCatalog[procedure]
	if !ok {
		return Rule{}, false
	}
	var class RuleClass
	switch rule.class {
	case accessPublicBounded:
		class = RuleClassPublicBounded
	case accessSessionLifecycle:
		class = RuleClassSessionLifecycle
	case accessProtected:
		class = RuleClassProtected
	}
	return Rule{Class: class, Action: rule.action, ResourceKind: rule.resourceKind, Mutating: rule.mutating}, true
}
