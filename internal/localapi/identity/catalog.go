package identity

import "ardents/internal/localapi/protocol/ardentsv1connect"

type accessClass uint8

const (
	accessPublicBounded accessClass = iota + 1
	accessSessionLifecycle
	accessProtected
)

type procedureRule struct {
	class  accessClass
	action string
}

var procedureCatalog = map[string]procedureRule{
	ardentsv1connect.IdentityServiceBeginAuthenticationProcedure:              {class: accessPublicBounded},
	ardentsv1connect.IdentityServiceCompleteAuthenticationProcedure:           {class: accessPublicBounded},
	ardentsv1connect.IdentityServiceEndSessionProcedure:                       {class: accessSessionLifecycle},
	ardentsv1connect.IdentityServiceEnrollFirstPrincipalProcedure:             {class: accessPublicBounded},
	ardentsv1connect.IdentityServiceEnrollPrincipalProcedure:                  {class: accessProtected, action: "identity.principal.enroll"},
	ardentsv1connect.IdentityServiceRevokeDeviceProcedure:                     {class: accessProtected, action: "identity.device.revoke"},
	ardentsv1connect.IdentityServiceListDeviceRevocationsProcedure:            {class: accessProtected, action: "identity.device-revocations.list"},
	ardentsv1connect.IdentityServiceIssueAccessGrantProcedure:                 {class: accessProtected, action: "identity.grant.issue"},
	ardentsv1connect.IdentityServiceRevokeAccessGrantProcedure:                {class: accessProtected, action: "identity.grant.revoke"},
	ardentsv1connect.IdentityServiceListAccessGrantsProcedure:                 {class: accessProtected, action: "identity.grant.list"},
	ardentsv1connect.IdentityServiceIssueApplicationEnrollmentTicketProcedure: {class: accessProtected, action: "identity.principal.enroll"},
	ardentsv1connect.IdentityServiceImportDelegationRevocationProcedure:       {class: accessPublicBounded},
}
