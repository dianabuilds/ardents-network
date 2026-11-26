// Package servingdeclaration owns the pure closed Source serving-declaration
// byte grammar: exact field names, duplicate/null refusal, one complete object
// and exact 32-byte array shapes, bounded to one finite input. It returns
// declaration facts only and opens no resource. State retains all signed
// authority/time/root decisions and processresource admits actual placement;
// this codec performs no State acceptance, root creation, signing or runtime
// policy.
//
// The runtime serve composition (cmd/ardents-next source input), canonical
// Control prepare-state/inspect-latest and State's read-only SourceInspection
// consume this one shared grammar. It mirrors the original node-role
// declaration codec in internal/domain/node (plan_declaration.go, #673): a shared
// byte grammar with no authority. Extracted from internal/network/source as
// AR-N-07a under the audit-remediation campaign (#691); the canonical
// declaration bytes are unchanged. Only the standard library is imported.
package servingdeclaration
